package main

import (
    "context"
    "encoding/json"
    "math/rand"
    "os"
    "strconv"
    "strings"
    "time"
    "log"
    "sync"
    "fmt"

    "github.com/go-redis/redis/v8"
    "github.com/google/uuid"
)

const _defaultUsers = 10

type Event struct {
    ID   int    `json:"id"`
    Data string `json:"data"`
    Resp string `json:"resp_q"`
}

func runAgent(stats *StatsReporter, client *redis.Client, frequencyMs int, streamName string) {

	var lastID int

	for {

		func() {

		start := time.Now()
		defer func(){ stats.AddService(&ServiceSummary{Name: "DEMO", Duration: time.Since(start), Success: true}) }()

		dataLen := rand.Intn(257) + 256
        data := make([]byte, dataLen)
        rand.Read(data)

        lastID++ // Increment the ID

	respQ := uuid.NewString()

	pubsub := client.Subscribe(context.Background(), respQ)
	defer pubsub.Close()

        // Create and send the event
	event := Event{ID: lastID, Data: string(data), Resp: respQ}
        eventJSON, _ := json.Marshal(event)

        client.XAdd(context.Background(), &redis.XAddArgs{
            Stream:       streamName,
            MaxLenApprox: 1000,  // Limit stream length
            ID:           "*",   // Server-generated ID
            Values:       map[string]interface{}{"data": eventJSON},
        }).Result()

        log.Printf("Added event with id: %d", lastID)

	_, err := pubsub.ReceiveMessage(context.Background())
        if err != nil {
            fmt.Println("Error receiving response:", err)
        }
	}()

        // Sleep for the specified frequency
        time.Sleep(time.Duration(frequencyMs) * time.Millisecond)	

	}

}

func main() {
    // Read parameters from command line
    streamName := os.Args[1]
    frequencyMs, _ := strconv.Atoi(os.Args[2])
    consumerGroups := strings.Split(os.Args[3], ",")
    users, _ := strconv.Atoi(os.Args[4])

    if users == 0 {
        users = _defaultUsers
    }

    var stats StatsReporter

    stats.Start([]string{"DEMO"}, 30)

    // Redis connection
    client := redis.NewClient(&redis.Options{
        Addr:     "redis:6379", // Assuming Redis is running in a Docker container named 'redis'
        Password: "",          // If your Redis instance has a password
        DB:       0,           // Default DB
    })

    // Create stream if it doesn't exist
    client.XGroupCreateMkStream(context.Background(), streamName, "mygroup", "$") // Initial group for stream creation

    // Add consumer groups
    for _, group := range consumerGroups {
        client.XGroupCreateMkStream(context.Background(), streamName, group, "$")
    }

    /*var lastID int

    // Infinite loop to produce events
    for {
        // Random string data generation
        dataLen := rand.Intn(257) + 256
        data := make([]byte, dataLen)
        rand.Read(data)

        lastID++ // Increment the ID

        // Create and send the event
        event := Event{ID: lastID, Data: string(data)}
        eventJSON, _ := json.Marshal(event)

        client.XAdd(context.Background(), &redis.XAddArgs{
            Stream:       streamName,
            MaxLenApprox: 1000,  // Limit stream length
            ID:           "*",   // Server-generated ID
            Values:       map[string]interface{}{"data": eventJSON},
        }).Result()

	log.Printf("Added event with id: %d", lastID)

        // Sleep for the specified frequency
        time.Sleep(time.Duration(frequencyMs) * time.Millisecond)
    }
    */

    var wg sync.WaitGroup

    wg.Add(1)

    for i := 0; i < users; i++ {

	go runAgent(&stats, client, frequencyMs, streamName)
    }

    wg.Wait()
}




type StatsReporter struct {
	IntervalSecs int
	mux          sync.RWMutex
	table        map[string]*internalStatsRec
}

type ServiceSummary struct {
	Name     string
	Duration time.Duration
	Success  bool
}

type internalStatsRec struct {
	max     time.Duration
	min     time.Duration
	total   time.Duration
	success int
	failure int
	inMux   sync.RWMutex
}

func (i *internalStatsRec) String() string {
	return fmt.Sprintf("Max: %v, Min:%v, Total: %v, Success: %d, Failure: %d", i.max, i.min, i.total, i.success, i.failure)
}

func (sr *StatsReporter) Start(services []string, interval int) {

	sr.IntervalSecs = interval

	sr.table = make(map[string]*internalStatsRec, len(services))

	for _, s := range services {
		sr.table[s] = &internalStatsRec{min: time.Hour * 10}
	}

	go sr.report()
}

func (sr *StatsReporter) report() {
	log.Printf("Stats reporter monitoring started")
	for {
		time.Sleep(time.Second * time.Duration(sr.IntervalSecs))

		sr.mux.Lock()

		for serv, stats := range sr.table {
			allreqs := stats.failure + stats.success
			if allreqs == 0 {
				// Info("STATS nothing to report for service %s", serv)
				continue
			}
			avgmicro := stats.total.Microseconds() / int64(allreqs)
			avgdur := time.Duration(avgmicro * int64(time.Microsecond))
			log.Printf("STATS Service [%s] Max[%s] Min[%s] Avg[%s] Success[%d] Failures[%d]", serv, stats.max.String(), stats.min.String(), avgdur.String(), stats.success, stats.failure)
			sr.table[serv] = &internalStatsRec{min: time.Hour * 10}
		}

		sr.mux.Unlock()

	}
}

func (sr *StatsReporter) asyncAddService(summary *ServiceSummary) {

	sr.mux.RLock()
	defer sr.mux.RUnlock()
	service := sr.table[summary.Name]

	if service != nil {
		service.inMux.Lock()
		defer service.inMux.Unlock()
		if summary.Duration > service.max {
			service.max = summary.Duration
		}
		if summary.Duration < service.min {
			service.min = summary.Duration
		}

		service.total += summary.Duration

		if summary.Success {
			service.success++
		} else {
			service.failure++
		}
	}
}

func (sr *StatsReporter) AddService(summary *ServiceSummary) {

	go sr.asyncAddService(summary)
}

