package dependency

import (
	"github.com/beeker1121/goque"
	// "io/ioutil"
	"log"
	"time"
	"github.com/Shopify/sarama"
	"encoding/json"
	"sync"
	//"gopkg.in/mgo.v2/bson"
//	"encoding/base64"
)

var q *goque.Queue

var lock sync.Mutex

const time_in_ms = 1000

const maxGoRoutines = 10

const dirQueuePath = "/Users/bandi.kishore/test/diskqueue/"

var guard = make(chan struct{}, maxGoRoutines)

var queueOfQueues = make([]*goque.Queue, 0)

var sizeSoFar = 0

var maxSize = 50 * 1024 * 1024 // 50 MB per queue

var firstTime = true

func InitDiskQueue() {
	var err error;
	q, err = goque.OpenQueue(dirQueuePath)
	if(err != nil) {
		log.Printf("Error occured while creating disk backed Queue at location %s with err %s", dirQueuePath, err)
		return
	}
	go startDeQueue()
}
	
func startDeQueue() {
	ticker := time.NewTicker(time.Millisecond*time_in_ms)
	for {
		select {
			case <-ticker.C:
					lastReadCount := q.Length()
					lastLoopCount := 0
					for{
						if(q.Length() == 0) {
							// log.Print("No topic Found")
							break
						} else {
							if(lastReadCount == q.Length()) {
								lastLoopCount++
							}
							// lastLoopCount it determines that the loop was continously looping for last 3 times with same number of messages.
							// Maybe down stream is failing all the time, so pause for a while (as long as ticket time)
							if(lastLoopCount >= 4) {
								break;
							}
							
							lastReadCount = q.Length()
							processMessageFromQueue()
						    
						    // log.Printf("Data unmarshalled was : %+v", event)
						}
					}
				
		}
	}
}

func processMessageFromQueue() {
	eventBinary, err := q.Dequeue()
    if(err != nil) {
        log.Printf("%s Error occured while reading topic from file", err)
        return
    }

	processMessageInParallel(eventBinary)
}

// Function would block few messages until the a goroutine is ready to process.
// This has a cascading effect of blocking even the main thread which reads from Queue.
func processMessageInParallel(item *goque.Item) {
	guard <- struct{}{} 
    go func(item *goque.Item) {
        processSingleMessage(item)
        <-guard
    }(item)
}

func processSingleMessage(item *goque.Item) {
	var event Event
	err := json.Unmarshal(item.Value, &event)
	if err != nil {
		log.Fatalf("%s Error occured while Unmarshaling diskData", err)
		return
	}
	// Kish - TODO : Do Event Validations.
	// Kish - TODO : Handle basic transformation which Spark does.
	// Kish - TODO : Send to awz_s3 topic.
	sendToKafka(event.Header.Name, item.Value)
}

// SendMessage Given a topic string and en event send it to the client
func sendMessage(topic string, data string) {
	// bData, _ := ioutil.ReadFile("/home/ubuntu/testgo/mytemp.json")
	// bData, _ := ioutil.ReadFile("/Users/bandi.kishore/test/Test.json")
	// bData, _ := json.Marshal(mydata)
	// fmt.Print("Enter text: %s",bData)
	// sendByteMessage(topic, bData)
	log.Printf("Data was : %+v", data)
	diskData := DiskData{topic, data}
	log.Printf("Data was : %+v", data)
	b, err := json.Marshal(&DiskData{topic, data})
	if err != nil {
        log.Printf("%s Error occured while Marshaling Disk Data", err)
        return
    }
	log.Printf("Binary Data written was : %+v", b)
	// q.Enqueue(b)
	
	// var diskData DiskData
	diskData = DiskData{}
    err = json.Unmarshal(b, &diskData)
    if err != nil {
	    	log.Printf("%s Error occured while Unmarshaling diskData", err)
	    	return;
    }
    
    log.Printf("Data unmarshalled was : %+v", diskData)
}

// SendMessage Given a topic string and en event send it to the client
func sendByteMessage(bData []byte) {
	// diskData := DiskData{topic, base64.StdEncoding.EncodeToString(bData)}
	q.Enqueue(bData)
}

func sendToKafka(topic string, bData []byte) {
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.StringEncoder(bData),
	}
	writeToKafka(msg, producer)
}

func closeDiskQueue() error {
	return q.Close()
}