# Authoring games

Everything on this page is built from dropdowns in the board editor. There is
no scripting language, and nothing here is specific to the bundled templates —
the templates use these features because every author can.

If you would rather read the finished article, the Monopoly template in
`frontend/src/lib/monopoly.ts` uses every feature below, and each rule is
commented with why it is written that way.

## The model

A game is a **board** and a set of **rules**.

The board is cells and directed edges between them. Each cell has a **type**
you define, **fields** you invent, and two action lists: `onLand` (the player
stopped here) and `onPass` (the player went through).

The rules declare the dice, the **resources** every player carries, the cell
types and the fields each type offers, and optionally items, equipment slots,
levels, hidden cells and free movement.

The engine knows nothing about properties, rent, monopolies or hit points. It
executes actions. Everything else is something an author wrote.

## Fields are your vocabulary

A field is any name and value you put on a cell: `cost`, `rent`, `damage`,
`group`, `faction`. Actions read fields by name, so the fields you invent
become the vocabulary the rest of your rules are written in.

This matters most for grouping. Give every blue street `group: blue` and every
brown street `group: brown`, and you can now ask questions about "the rest of
this group" without ever naming a colour in a rule.

## Asking about other cells

Most actions work on the cell they are attached to. **Which cells** — a cell
query — lets an action look at the whole board.

A query has four filters, all optional:

| Filter | Means |
| --- | --- |
| **Cell type** | only cells of this type |
| **Owned by** | `nobody`, `this player`, `the owner of this cell`, `another player`, or anyone |
| **Field** + **Value** | only cells whose field holds that value |
| **Field** + **same value as this cell** | only cells matching *this* cell's value for that field |
| **Level at least** | only cells built up to at least that level |
| **not counting this cell** | leave the cell being resolved out |

**"Owned by" is about the square, not the visitor.** When a player lands on
somebody else's property, "this player" is the *visitor*. To scale rent by what
the landlord owns, choose **the owner of this cell**. Mixing these two up is
the easiest mistake to make here, and it produces a rule that looks right and
charges the wrong amount.

Three actions use queries:

- **If enough cells match…** — compares the count against a number, or against
  another count.
- **For every matching cell…** — runs a list of actions once for each match,
  with that cell as the context. `Set cell level` inside the loop sets the
  level of the matched cell, not of the square the player is standing on.
- **A computed amount** with a term of kind *how many cells match* — turns a
  count into a number you can multiply by.

### Recipe: double rent for a complete colour group

On a street's `onLand`, inside the "nobody has built here" branch:

```
If enough cells match…
  Which cells:  type = property, field group = same value as this cell,
                owned by = the owner of this cell
  Amount:       computed → base = how many cells match
                           (type = property, field group = same as this cell)
  Then:  charge rent × 2
  Else:  charge rent
```

Both sides are counts, which is the point: the rule says "the owner holds every
square in this group", not "the owner holds three squares". Add a fourth street
to the group and the rule still holds.

### Recipe: rent that multiplies by holdings

For stations, or anything else worth more in a set:

```
Transfer resource: money, to the cell owner
  Amount: computed
    base:  this cell's rent field
    × by:  how many cells match (type = station, owned by = the owner of this cell)
```

One station charges the base rent, four charge four times it, and nothing had
to be written out per tier.

### Recipe: pay per built square

A chance card that reads the whole board:

```
For every matching cell…
  Which cells: type = property, owned by = this player, level at least 1
  Then:        lose 40 money
  If none:     log "Nothing built, nothing to repair."
```

## Computed amounts

Any action that takes an amount can compute it instead. The shape is fixed, and
so is the order:

```
base  (+ plus)  (- minus)  (× times)  (÷ divided by)  then min, then max
```

Each of `base`, `plus`, `minus`, `times` and `divided by` is a **term**: a
number, a field on this cell, one of the player's stats (with equipment, or
without), or how many cells match a query. `min` and `max` are plain numbers,
because a clamp is a rule of the game rather than a quantity to work out.

Division by zero is ignored rather than fatal — a count that comes to zero
leaves the value alone instead of ending the game.

This is a fixed shape rather than an expression tree on purpose. It stays a
handful of dropdowns, and the person reading your game a year later gets the
same arithmetic you did without hunting for brackets.

## Auctions

**Auction to all players** puts something in front of the whole table. Bidding
goes round in turn: each player is offered a few raises and "pass", passing
takes them out, and the last player still in wins at their bid.

What you fill in:

| Field | Means |
| --- | --- |
| **Bid with** | which resource is money here |
| **Amount** | the opening bid — a number, a field like `mortgageValue`, or a computed amount |
| **Smallest raise** | the increment; leave it blank for a tenth of the opening bid |
| **Who may bid** | everyone at the table, or everyone except the player whose turn it is |
| **The winner gets** | actions that run **as the winner** |
| **If nobody bids** | actions that run when everybody passes |

The winner pays their bid to the bank automatically, and then your "winner
gets" actions run. Because they run as the winner, `Set cell owner → current
player` gives the square to whoever won the bidding.

A player who cannot afford the next raise is dropped from the auction rather
than shown a prompt whose only answer is "pass". If nobody bids at all, the
"if nobody bids" branch runs and nothing changes hands.

Bidding is stepped rather than free-form: the server generates the amounts, and
accepts only an amount it offered, re-checked against the player's balance when
the answer arrives. A client cannot bid money it does not have.

The auction is part of the saved game, so a player who reloads in the middle of
one comes back to the same bidding, and a room that replays its event journal
replays the auction with it.

## Trading

Trading is separate from auctions and works between two players: the player
whose turn it is builds an offer, and the other player accepts or declines.
Both sides are re-checked when the offer is accepted, because turns can pass in
between and nobody should be able to promise what they no longer hold.

An auction is the many-player version; a trade is the two-player one where each
side gives something.

## What is still not possible

- **No table lookups.** Tiered values are written as a descending chain of
  "if level at least N" checks.
- **No free-form bids.** An auction offers stepped amounts, not a text box.
- **Nothing reads the dice after the move.** A rule cannot charge "four times
  the roll".
- **No author-supplied code.** By design: this is a self-hosted server that
  runs other people's games.

## Publication catches mistakes early

Validation runs when you publish, and it rejects the mistakes that would
otherwise be silent at play time: a query naming a cell type that does not
exist, an owner filter that is not one of the five, a field value with no field
to compare against, an auction with no currency or no prize, an item ID that
matches no item, a teleport to a missing cell.

A broken query does not fail loudly — it simply matches nothing, and the rent
comes to zero. That is why these are refused at publication rather than left
for a player to discover.
