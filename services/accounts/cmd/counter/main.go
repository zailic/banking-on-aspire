package main

import (
	"log"
	"net/http"

	"dev.local/banking-on-aspire/services/accounts/actors"
	daprd "github.com/dapr/go-sdk/service/http"
)

func main() {
	service := daprd.NewService(":8080")
	service.RegisterActorImplFactoryContext(actors.CounterActorFactory)

	log.Printf(
		"starting actor service on :8080; actor type=%s",
		actors.CounterActorType,
	)

	if err := service.Start(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("actor service failed: %v", err)
	}
}
