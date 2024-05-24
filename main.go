package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/quic-go/quic-go/http3"
	"github.com/quic-go/webtransport-go"
)

type MessageServer struct {
	listeners []chan []byte
}

func (m *MessageServer) Subscribe() chan []byte {
	ch := make(chan []byte)
	m.listeners = append(m.listeners, ch)
	return ch
}

func (m *MessageServer) Unsubscribe(ch chan []byte) {
	for i := range m.listeners {
		if m.listeners[i] == ch {
			m.listeners = m.listeners[:i+copy(m.listeners[i:], m.listeners[i+1:])]
			close(ch)
			break
		}
	}
}

func (m *MessageServer) Broadcast(message []byte) {
	for _, ch := range m.listeners {
		ch <- message
	}
}

func main() {
	messageServer := &MessageServer{
		listeners: make([]chan []byte, 0),
	}

	wt := webtransport.Server{
		H3: http3.Server{
			Addr: ":4433",
		},
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	sessionID := 0

	http.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		session, err := wt.Upgrade(w, r)
		if err != nil {
			log.Printf("upgrading failed: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)

		sessionID += 1
		log.Printf("Session #%d start.", sessionID)

		messageCh := messageServer.Subscribe()
		closeCh := make(chan int)
		wg := &sync.WaitGroup{}

		// メッセージを送信する
		wg.Add(1)
		go (func() {
			defer wg.Done()
			for {
				select {
				case message := <-messageCh:
					log.Printf("Send message: %s\n", message)
					stream, err := session.OpenUniStream()
					if err != nil {
						log.Println("Open stream failed:", err)
						break
					}
					_, err = stream.Write(message)
					if err != nil {
						log.Println("Send stream failed:", err)
						break
					}
					stream.Close()
				case <-closeCh:
					log.Println("Send stream closed.")
					return
				}
			}
		})()

		// メッセージを受信する
		wg.Add(1)
		go (func() {
			defer wg.Done()
			for {
				acceptCtx, acceptCtxCancel := context.WithTimeout(session.Context(), 10*time.Minute)
				stream, err := session.AcceptUniStream(acceptCtx)
				if err != nil {
					acceptCtxCancel()
					log.Println("Accept stream failed:", err)
					break
				}
				acceptCtxCancel()
				p, err := io.ReadAll(stream)
				if err != nil {
					log.Println("Session closed, ending stream listener:", err)
					break
				}
				log.Printf("Received stream: %s", p)
				messageServer.Broadcast(p)
			}
			closeCh <- 1
		})()

		wg.Wait()
		messageServer.Unsubscribe(messageCh)
		log.Printf("Session #%d closed.", sessionID)
	})

	wt.ListenAndServeTLS("localhost.pem", "localhost-key.pem")
}
