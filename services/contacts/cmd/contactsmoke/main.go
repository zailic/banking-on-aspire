package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	contactsv1 "github.com/zailic/banking-on-aspire/platform/gen/go/banking/contacts/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

func main() {
	endpoint := flag.String("endpoint", "", "Contacts gRPC endpoint")
	action := flag.String("action", "get", "create, get, list, update, or delete")
	parent := flag.String(
		"parent",
		"users/smoke-user",
		"Canonical user resource that owns the contact",
	)
	contactID := flag.String("contact-id", "persistence-smoke", "Contact resource ID")
	internalAccount := flag.String(
		"internal-account",
		"",
		"Existing internal account used by the create action",
	)
	flag.Parse()
	token := strings.TrimSpace(os.Getenv("CONTACTS_SMOKE_TOKEN"))
	if strings.TrimSpace(*endpoint) == "" {
		log.Fatal("-endpoint is required")
	}

	target := strings.TrimPrefix(*endpoint, "grpc://")
	connection, err := grpc.NewClient(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("connect to Contacts: %v", err)
	}
	defer connection.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if token != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
	}
	client := contactsv1.NewContactsServiceClient(connection)
	name := strings.TrimRight(*parent, "/") + "/contacts/" + *contactID

	switch *action {
	case "create":
		if strings.TrimSpace(*internalAccount) == "" {
			log.Fatal("-internal-account is required for the create action")
		}
		contact, err := client.CreateContact(ctx, &contactsv1.CreateContactRequest{
			Parent:    *parent,
			ContactId: *contactID,
			Contact: &contactsv1.Contact{
				DisplayName: "Persistence Smoke",
				Destination: &contactsv1.Contact_InternalAccount{InternalAccount: *internalAccount},
			},
		})
		if err != nil {
			log.Fatalf("create contact: %v", err)
		}
		fmt.Printf("created %s etag=%s\n", contact.GetName(), contact.GetEtag())
	case "get":
		contact, err := client.GetContact(ctx, &contactsv1.GetContactRequest{Name: name})
		if err != nil {
			log.Fatalf("get contact: %v", err)
		}
		fmt.Printf(
			"found %s display_name=%q etag=%s\n",
			contact.GetName(),
			contact.GetDisplayName(),
			contact.GetEtag(),
		)
	case "list":
		response, err := client.ListContacts(ctx, &contactsv1.ListContactsRequest{Parent: *parent})
		if err != nil {
			log.Fatalf("list contacts: %v", err)
		}
		fmt.Printf("listed %d contacts\n", len(response.GetContacts()))
		for _, contact := range response.GetContacts() {
			fmt.Printf(
				"- %s display_name=%q etag=%s\n",
				contact.GetName(),
				contact.GetDisplayName(),
				contact.GetEtag(),
			)
		}
	case "update":
		current, err := client.GetContact(ctx, &contactsv1.GetContactRequest{Name: name})
		if err != nil {
			log.Fatalf("get contact for update: %v", err)
		}
		updated, err := client.UpdateContact(ctx, &contactsv1.UpdateContactRequest{
			Contact: &contactsv1.Contact{
				Name:        current.GetName(),
				DisplayName: "Persistence Smoke Updated",
				Etag:        current.GetEtag(),
			},
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"display_name"}},
		})
		if err != nil {
			log.Fatalf("update contact: %v", err)
		}
		fmt.Printf(
			"updated %s display_name=%q etag=%s\n",
			updated.GetName(),
			updated.GetDisplayName(),
			updated.GetEtag(),
		)
	case "delete":
		if _, err := client.DeleteContact(
			ctx,
			&contactsv1.DeleteContactRequest{Name: name},
		); err != nil {
			log.Fatalf("delete contact: %v", err)
		}
		fmt.Printf("deleted %s\n", name)
	default:
		log.Fatalf("unsupported -action %q", *action)
	}
}
