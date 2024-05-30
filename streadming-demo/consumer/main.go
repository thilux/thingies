package main

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "time"
    "strconv"

    "github.com/go-redis/redis/v8"
    "github.com/google/uuid"
)

type Event struct {
    ID   int    `json:"id"`
    Data string `json:"data"`
    Resp string `json:"resp_q"`
}

func main() {
    // Read parameters from command line
    streamName := os.Args[1]
    consumerGroupName := os.Args[2]
    rate, _ := strconv.Atoi(os.Args[3])

    // Redis connection
    client := redis.NewClient(&redis.Options{
        Addr:     "redis:6379", // Assuming Redis is running in a container named 'redis'
        Password: "",          // If your Redis instance has a password
        DB:       0,           // Default DB
    })

    consumerID := uuid.NewString()

    fmt.Printf("Consumer %s starting...\n", consumerID)

    // Infinite loop to consume events
    for {
        // Read the next event from the stream
        streams, err := client.XReadGroup(context.Background(), &redis.XReadGroupArgs{
            Group:    consumerGroupName,
            Consumer: consumerID, // Unique consumer name
            Streams:  []string{streamName, ">"},      // Read from the last acknowledged ID onwards
            Count:    1,                               // Read one event at a time
            Block:    0,                               // Don't block indefinitely
        }).Result()

        if err != nil || len(streams) == 0 || len(streams[0].Messages) == 0 {
            // No new events, wait for a bit
            time.Sleep(100 * time.Millisecond)
            continue
        }

        msg := streams[0].Messages[0]
        eventData := msg.Values["data"].(string) // Extract the event data

        // Parse the event JSON
        var event Event
        json.Unmarshal([]byte(eventData), &event)

        // Print the consumed event ID
        fmt.Printf("Consuming event %d\n", event.ID)

	var resp string

	for i := 0; i < len(event.Data) * rate; i++ {
		if i %2 == 0 && i % 1234 == 0 {
			resp += "hit the pot\n"
		}
	}


        // Acknowledge the event
        client.XAck(context.Background(), streamName, consumerGroupName, msg.ID).Result()

	 client.Publish(context.Background(), event.Resp, fmt.Sprintf("Processed event %d - %s", event.ID, resp)).Err()
    }
}

