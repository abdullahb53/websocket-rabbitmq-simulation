package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	amqp "github.com/rabbitmq/amqp091-go"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func reader(conn *websocket.Conn, rmq *RabbMQ) {

	ch, _ := rmq.conn.Channel()
	// if err := ch.Qos(20, 0, false); err != nil {
	// 	log.Println(err)
	// }
	defer ch.Close()

	q := getRbbmQueue("fruits4", ch)
	msgs, err := ch.Consume(
		q.Name, // queue name
		"",     // consumer name (empty for auto-generated name)
		false,  // autoAck: false (manual acknowledgment)
		false,  // exclusive: false (queue can be accessed by multiple consumers)
		false,  // no-local: false (do not deliver own messages)
		false,  // no-wait: false (wait for the server's response)
		nil,
	)
	if err != nil {
		log.Fatalf("consuming err:%v", err)
	}

	for {
		log.Println("Reader executed!")
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				fmt.Println("WebSocket connection closed unexpectedly:", err)
			} else {
				fmt.Println("WebSocket read error:", err)
			}
			break
		}

		allMsg := string(p)
		log.Println(strings.ContainsAny("/", allMsg), "-", allMsg)
		if strings.ContainsAny("/", allMsg) && len(strings.Split(allMsg, "/")) == 2 {

			headerMsg, contentMsg := strings.Split(allMsg, "/")[0], strings.Split(allMsg, "/")[1]
			fmt.Println("Header: ", headerMsg)

			if headerMsg == "addFruit" {

				err = ch.PublishWithContext(context.Background(),
					"",     // exchange name (empty for direct exchange)
					q.Name, // routing key (queue name is used as the routing key for direct exchange)
					false,  // mandatory: false (if set to true, the server will return an error if the message cannot be routed to a queue)
					false,  // immediate: false (if set to true, the server will return an error if there are no consumers for the message)
					amqp.Publishing{
						ContentType: "text/plain",
						Body:        []byte(contentMsg),
					},
				)
				if err != nil {
					log.Fatalf("Failed to publish message: %v", err)
				}

				conn.WriteMessage(messageType, []byte("addFruit/"+contentMsg))

			} else if headerMsg == "collectFruit" {
				log.Println("collect fruit golang page@@@")

				go func(conn *websocket.Conn) {
					ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
					defer cancel()
				breakup:
					for {
						select {
						case d := <-msgs:
							print("@@@@@@@@@@@@@@@@@@\n")

							err := conn.WriteMessage(messageType, []byte("collectFruit"))
							if err != nil {
								log.Println(err)
								cancel()
							}
							// Manually acknowledge the message
							if err := d.Ack(false); err != nil {
								log.Printf("Failed to acknowledge message: %v", err)
							}
							break breakup

						case <-ctx.Done():

							break breakup
						}
					}

				}(conn)

			} else if headerMsg == "allFruit" {

				go func(conn *websocket.Conn) {
					ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
					defer cancel()

					bigPacket := ""
					counter := 0
					bigArray := make([][]byte, 0)

				breakup:
					for {

						select {
						case d := <-msgs:
							counter++
							bigPacket = bigPacket + string(d.Body) + ","
							if err := d.Ack(false); err != nil {
								log.Println(err)
							}
							bigArray = append(bigArray, d.Body)

						case <-ctx.Done():
							log.Println("bigPacket:", bigPacket)
							if len(bigPacket) > 1 {
								print("COUNT @@@ ::: \n")
								print(counter, "\n")
								err := conn.WriteMessage(messageType, []byte("allFruit/"+bigPacket[:len(bigPacket)-1]))

								for i := 0; i < len(bigArray); i++ {
									err = ch.PublishWithContext(context.Background(),
										"",     // exchange name (empty for direct exchange)
										q.Name, // routing key (queue name is used as the routing key for direct exchange)
										false,  // mandatory: false (if set to true, the server will return an error if the message cannot be routed to a queue)
										false,  // immediate: false (if set to true, the server will return an error if there are no consumers for the message)
										amqp.Publishing{
											ContentType: "text/plain",
											Body:        bigArray[i],
										},
									)
									if err != nil {
										log.Fatalf("Failed to publish message: %v", err)
									}

								}

								if err != nil {
									log.Println("connWriterr:", err)
									break breakup
								}
							}
							break breakup

						}
					}

				}(conn)

			} else {
				log.Println("Wrong type of message")
			}
		}

		// if err := conn.WriteMessage(messageType, p); err != nil {
		// 	log.Println(err)
		// }

	}
}

type InjectRabbitToHandler func(http.ResponseWriter, *http.Request)

func NewInjectRabbitToHandler(rbmqCh *RabbMQ) InjectRabbitToHandler {
	return func(rw http.ResponseWriter, r *http.Request) {
		upgrader.CheckOrigin = func(r *http.Request) bool { return true }

		ws, err := upgrader.Upgrade(rw, r, nil)
		if err != nil {
			log.Println(err)
		}

		reader(ws, rbmqCh)
	}
}

type RabbMQ struct {
	conn  *amqp.Connection
	close chan struct{}
}

func getRbbmQueue(q string, ch *amqp.Channel) amqp.Queue {
	realQueue, err := ch.QueueDeclare(
		q,     // queue name
		false, // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		log.Printf("Failed to declare a queue [%v], error: %x", q, err)
	}
	return realQueue
}

func newRabbMQ() *RabbMQ {
	conn, err := amqp.Dial("amqp://admin:admin@localhost:5672/")
	if err != nil {
		log.Fatal("RabbitMQ connection is failed.")
	}

	return &RabbMQ{
		conn:  conn,
		close: make(chan struct{}, 1),
	}

}

func homePage(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Home Page")
}

func main() {

	rmq := newRabbMQ()
	defer rmq.conn.Close()
	// ch, err := rmq.conn.Channel()
	// if err != nil {
	// 	log.Fatalf("connection channel error: %v", err)
	// }

	// Injection to handler with rabbitmq channel.
	http.HandleFunc("/", homePage)
	http.HandleFunc("/ws", NewInjectRabbitToHandler(rmq))
	fmt.Println("Serving..")
	log.Fatal(http.ListenAndServe(":40123", nil))

}
