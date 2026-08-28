package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	usersv1 "dev.local/banking-on-aspire/platform/gen/go/banking/users/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

func main() {
	endpoint := flag.String("endpoint", "", "Users gRPC endpoint reported by Aspire")
	action := flag.String("action", "resolve", "resolve or get")
	name := flag.String("name", "", "user resource name for get")
	flag.Parse()
	token := strings.TrimSpace(os.Getenv("USERS_SMOKE_TOKEN"))
	if *endpoint == "" || token == "" {
		log.Fatal("-endpoint and USERS_SMOKE_TOKEN are required")
	}

	target := strings.TrimPrefix(strings.TrimPrefix(*endpoint, "grpc://"), "http://")
	connection, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}
	defer connection.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
	client := usersv1.NewUsersServiceClient(connection)

	var user *usersv1.User
	switch *action {
	case "resolve":
		user, err = client.GetOrCreateCurrentUser(ctx, &usersv1.GetOrCreateCurrentUserRequest{})
	case "get":
		if *name == "" {
			log.Fatal("-name is required for get")
		}
		user, err = client.GetUser(ctx, &usersv1.GetUserRequest{Name: *name})
	default:
		log.Fatalf("unsupported action %q", *action)
	}
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s username=%q display_name=%q status=%s etag=%s\n",
		user.GetName(), user.GetUsername(), user.GetDisplayName(), user.GetStatus(), user.GetEtag())
}
