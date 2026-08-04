package game

import (
	"encoding/json"
	"strings"
	"testing"
)

func hiddenDefinition() *GameDefinition {
	definition := rpgDefinition()
	definition.Rules.HiddenCells = true
	for i := range definition.Board.Cells {
		if definition.Board.Cells[i].ID == "cave" {
			definition.Board.Cells[i].Title = "Dragon's Lair"
			definition.Board.Cells[i].Visual = CellVisual{BaseColor: "#8B0000"}
			definition.Board.Cells[i].Fields = map[string]any{"treasure": 999}
			definition.Board.Cells[i].OnLand = []ActionDefinition{
				{Type: "lose_resource", Resource: "health", Amount: intPtr(10)},
			}
		}
	}
	return definition
}

func TestOnlyTheStartingCellIsFaceUpAtTheBeginning(t *testing.T) {
	session := StartSession(hiddenDefinition(), []PlayerConfig{{Name: "Ada", Color: "#111"}, {Name: "Bob", Color: "#222"}})

	if !session.State.CellStates["start"].Revealed {
		t.Fatal("the cell everybody starts on should never be a surprise")
	}
	for _, id := range []string{"armoury", "cave", "summit"} {
		if session.State.CellStates[id].Revealed {
			t.Fatalf("cell %s started face up", id)
		}
	}
}

// The point of a hidden cell is that the client cannot know what is on it.
// Serialising the session is the only way it ever reaches a browser, so that
// is where this has to hold.
func TestHiddenCellContentsNeverReachTheClient(t *testing.T) {
	session := StartSession(hiddenDefinition(), []PlayerConfig{{Name: "Ada", Color: "#111"}, {Name: "Bob", Color: "#222"}})

	encoded, err := json.Marshal(session)
	if err != nil {
		t.Fatal(err)
	}
	wire := string(encoded)

	for _, secret := range []string{"Dragon's Lair", "8B0000", "treasure", "999"} {
		if strings.Contains(wire, secret) {
			t.Fatalf("the serialised session leaks %q from a face-down cell", secret)
		}
	}
	// The map's shape is not the secret: positions must still be present so the
	// board can be drawn.
	if !strings.Contains(wire, `"__hidden"`) {
		t.Fatal("hidden cells should report a placeholder type the interface can draw")
	}
}

func TestLandingOnACellTurnsItOverAndThenItStaysVisible(t *testing.T) {
	session := StartSession(hiddenDefinition(), []PlayerConfig{{Name: "Ada", Color: "#111"}, {Name: "Bob", Color: "#222"}})
	player := &session.State.Players[0]

	// Walk to the lair.
	session.executeOneAction(ActionDefinition{Type: "move_player_to", To: "cave"}, player, session.Definition.Board.getCellByID("start"))

	if !session.State.CellStates["cave"].Revealed {
		t.Fatal("landing on a cell did not turn it over")
	}
	// Its actions ran, which is the whole point of revealing on arrival.
	if player.Resources["health"] != 10 {
		t.Fatalf("health = %d, want the lair's own actions to have run", player.Resources["health"])
	}

	encoded, _ := json.Marshal(session)
	if !strings.Contains(string(encoded), "Dragon's Lair") {
		t.Fatal("a revealed cell should now be sent to the client")
	}
}

func TestScoutingRevealsAheadWithinRange(t *testing.T) {
	session := StartSession(hiddenDefinition(), []PlayerConfig{{Name: "Ada", Color: "#111"}, {Name: "Bob", Color: "#222"}})
	player := &session.State.Players[0]
	start := session.Definition.Board.getCellByID("start")

	session.executeOneAction(ActionDefinition{Type: "reveal_cells", Amount: intPtr(2)}, player, start)

	// start -> armoury -> cave is two steps; summit is three.
	if !session.State.CellStates["armoury"].Revealed || !session.State.CellStates["cave"].Revealed {
		t.Fatalf("cell states = %#v, want two steps ahead revealed", session.State.CellStates)
	}
	if session.State.CellStates["summit"].Revealed {
		t.Fatal("scouting reached further than its range")
	}
}

func TestScoutingASpecificCell(t *testing.T) {
	session := StartSession(hiddenDefinition(), []PlayerConfig{{Name: "Ada", Color: "#111"}, {Name: "Bob", Color: "#222"}})
	player := &session.State.Players[0]
	start := session.Definition.Board.getCellByID("start")

	events := session.executeOneAction(ActionDefinition{Type: "reveal_cells", To: "summit"}, player, start)
	if !session.State.CellStates["summit"].Revealed {
		t.Fatal("targeted scouting did not reveal the cell")
	}
	if len(events) == 0 {
		t.Fatal("targeted scouting produced no events")
	}

	// Revealing something already face up is a no-op rather than noise.
	if again := session.executeOneAction(ActionDefinition{Type: "reveal_cells", To: "summit"}, player, start); len(again) != 0 {
		t.Fatalf("events = %#v, want nothing for an already revealed cell", again)
	}
}

// A game that does not use hidden cells must be completely unaffected.
func TestGamesWithoutHiddenCellsAreUntouched(t *testing.T) {
	session := rpgSession(t)
	player := &session.State.Players[0]
	start := session.Definition.Board.getCellByID("start")

	if events := session.executeOneAction(ActionDefinition{Type: "reveal_cells", Amount: intPtr(3)}, player, start); events != nil {
		t.Fatalf("events = %#v, want reveal to do nothing without hidden cells", events)
	}

	encoded, _ := json.Marshal(session)
	if !strings.Contains(string(encoded), "Armoury") {
		t.Fatal("a normal game lost its cell titles")
	}
	if strings.Contains(string(encoded), "__hidden") {
		t.Fatal("a normal game gained hidden placeholders")
	}
}

// A session without a definition attached is a normal intermediate state when
// a room snapshot is being assembled, and marshalling one must not panic.
func TestMarshallingASessionWithNoDefinitionDoesNotPanic(t *testing.T) {
	for _, session := range []*GameSession{{}, {ID: "partial", Mode: "room"}} {
		if _, err := json.Marshal(session); err != nil {
			t.Fatalf("marshalling %#v failed: %v", session, err)
		}
	}
}
