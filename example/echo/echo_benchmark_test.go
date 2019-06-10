package main_test

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rsocket/rsocket-go"
	"github.com/rsocket/rsocket-go/internal/common"
	"github.com/rsocket/rsocket-go/payload"
	"github.com/rsocket/rsocket-go/rx"
	"github.com/stretchr/testify/assert"
)

const uri = "tcp://127.0.0.1:7878"

func TestClient_RequestResponse(t *testing.T) {
	client := createClient(uri)
	defer func() {
		_ = client.Close()
	}()
	wg := &sync.WaitGroup{}
	n := 5000000
	wg.Add(n)
	data := []byte(common.RandAlphanumeric(4096))

	now := time.Now()
	ctx := context.Background()
	for i := 0; i < n; i++ {
		m1 := []byte(fmt.Sprintf("benchmark_test_%d", i))
		client.RequestResponse(payload.New(data, m1)).
			SubscribeOn(rx.ElasticScheduler()).
			DoOnSuccess(func(ctx context.Context, s rx.Subscription, elem payload.Payload) {
				assert.Equal(t, data, elem.Data(), "data doesn't match")
				//m2, _ := elem.MetadataUTF8()
				//assert.Equal(t, m1, m2, "metadata doesn't match")
				wg.Done()
			}).
			Subscribe(ctx)
	}
	wg.Wait()
	cost := time.Since(now)
	log.Println(n, "COST:", cost)
	log.Println(n, "QPS:", float64(n)/cost.Seconds())

	assert.Equal(t, 0, common.CountByteBuffer(), "bytebuff leak")
}

func TestClients_RequestResponse(t *testing.T) {
	log.Println("---------------")
	doOnce(10000)
}

func createClient(uri string) rsocket.Client {
	client, err := rsocket.Connect().
		SetupPayload(payload.NewString("你好", "世界")).
		Acceptor(func(socket rsocket.RSocket) rsocket.RSocket {
			return rsocket.NewAbstractSocket(
				rsocket.RequestResponse(func(p payload.Payload) rx.Mono {
					log.Println("rcv reqresp from server:", p)
					if bytes.Equal(p.Data(), []byte("ping")) {
						return rx.JustMono(payload.NewString("pong", "from client"))
					}
					return rx.JustMono(p)
				}),
			)
		}).
		Transport(uri).
		Start(context.Background())
	if err != nil {
		panic(err)
	}
	return client
}

func doOnce(totals int) {
	wg := &sync.WaitGroup{}
	wg.Add(totals)
	data := []byte(strings.Repeat("A", 4096))
	md := []byte("benchmark_test")
	ctx := context.Background()
	clients := make([]rsocket.Client, totals)
	now := time.Now()
	for i := 0; i < totals; i++ {
		clients[i] = createClient(uri)
	}
	log.Println("SETUP:", time.Since(now))
	now = time.Now()
	for _, client := range clients {
		client.RequestResponse(payload.New(data, md)).
			DoFinally(func(ctx context.Context, sig rx.SignalType) {
				wg.Done()
			}).
			SubscribeOn(rx.ElasticScheduler()).
			Subscribe(ctx)
	}
	wg.Wait()
	cost := time.Since(now)

	log.Println("TOTALS:", totals)
	log.Println("COST:", cost)
	log.Printf("QPS: %.2f\n", float64(totals)/cost.Seconds())
	time.Sleep(10 * time.Hour)
	for _, client := range clients {
		_ = client.Close()
	}
}