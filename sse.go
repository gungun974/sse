package sse

import (
	"fmt"
	"net/http"
	"sync"
)

type ServerSideEventsServer interface {
	PublishTopic(topic string, event string, data string)

	HandleSSE(w http.ResponseWriter, r *http.Request)
}

func NewServerSideEventsServer() ServerSideEventsServer {
	return &ImplServerSideEventsServer{}
}

type ImplServerSideEventsServer struct{}

type serverSideEvent struct {
	Topic string
	Event string
	Data  string
}

var (
	sseLock    sync.Mutex
	sseClients []chan serverSideEvent
)

func (im *ImplServerSideEventsServer) PublishTopic(topic string, event string, data string) {
	sseLock.Lock()
	for _, sseClient := range sseClients {
		if sseClient != nil {
			sseClient <- serverSideEvent{
				Topic: topic,
				Event: event,
				Data:  data,
			}
		}
	}
	sseLock.Unlock()
}

func (im *ImplServerSideEventsServer) HandleSSE(w http.ResponseWriter, r *http.Request) {
	topic := r.URL.Query().Get("topic")

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	sseClient := make(chan serverSideEvent)

	sseLock.Lock()
	sseClients = append(sseClients, sseClient)
	sseLock.Unlock()

	defer func() {
		sseLock.Lock()
		for i, client := range sseClients {
			if client == sseClient {
				sseClients = append(sseClients[:i], sseClients[i+1:]...)
				break
			}
		}
		sseLock.Unlock()
		close(sseClient)
	}()

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Failed to setup SSE flusher", http.StatusInternalServerError)
		return
	}

	for {
		select {
		case data := <-sseClient:
			if data.Topic == topic {
				fmt.Fprintf(w, "event: %s\n", data.Event)
				fmt.Fprintf(w, "data: %s\n\n", data.Data)
				flusher.Flush()
			}
		case <-r.Context().Done():
			return
		}
	}
}
