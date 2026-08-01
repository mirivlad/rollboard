package main

import (
	"bufio"
	"net"
	"net/http"
	"testing"
)

func TestLoggingResponseWriterPreservesHijacker(t *testing.T) {
	wrapped := &loggingResponseWriter{ResponseWriter: hijackableResponseWriter{}, statusCode: http.StatusOK}
	connection, _, err := wrapped.Hijack()
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
}

type hijackableResponseWriter struct{}

func (hijackableResponseWriter) Header() http.Header       { return make(http.Header) }
func (hijackableResponseWriter) Write([]byte) (int, error) { return 0, nil }
func (hijackableResponseWriter) WriteHeader(int)           {}
func (hijackableResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	server, client := net.Pipe()
	go client.Close()
	return server, bufio.NewReadWriter(bufio.NewReader(server), bufio.NewWriter(server)), nil
}
