package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	accountsv1 "dev.local/banking-on-aspire/platform/gen/go/banking/accounts/v1"
	"google.golang.org/genproto/googleapis/type/money"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

func main() {
	endpoint := flag.String("endpoint", "", "Accounts gRPC endpoint")
	action := flag.String("action", "list", "list, deposit, or send")
	parent := flag.String("parent", "", "User parent for list or source account for deposit/send")
	beneficiary := flag.String("beneficiary", "", "Beneficiary resource for send")
	amount := flag.String("amount", "1.00", "Positive decimal amount")
	currency := flag.String("currency", "RON", "ISO currency code")
	requestID := flag.String("request-id", "", "Idempotency key; generated when omitted")
	flag.Parse()

	if strings.TrimSpace(*endpoint) == "" || strings.TrimSpace(*parent) == "" {
		log.Fatal("-endpoint and -parent are required")
	}
	token := strings.TrimSpace(os.Getenv("ACCOUNTS_SMOKE_TOKEN"))
	if token == "" {
		log.Fatal("ACCOUNTS_SMOKE_TOKEN is required")
	}
	value, err := parseMoney(*amount, *currency)
	if err != nil {
		log.Fatal(err)
	}
	if strings.TrimSpace(*requestID) == "" {
		*requestID = fmt.Sprintf("observability-%d", time.Now().UnixNano())
	}

	connection, err := grpc.NewClient(strings.TrimPrefix(*endpoint, "grpc://"), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("connect to Accounts: %v", err)
	}
	defer connection.Close()

	traceParent := newTraceParent()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token, "traceparent", traceParent)
	client := accountsv1.NewAccountsServiceClient(connection)

	switch *action {
	case "list":
		response, err := client.ListAccounts(ctx, &accountsv1.ListAccountsRequest{Parent: *parent, PageSize: 50})
		if err != nil {
			log.Fatalf("list accounts: %v", err)
		}
		for _, account := range response.GetAccounts() {
			fmt.Printf("%s\t%s\t%s\n", account.GetName(), account.GetCurrencyCode(), account.GetDisplayName())
		}
	case "deposit":
		deposit, err := client.DepositFunds(ctx, &accountsv1.DepositFundsRequest{
			Parent: *parent, Amount: value, Reference: "Observability drill", RequestId: *requestID,
		})
		if err != nil {
			log.Fatalf("deposit funds: %v", err)
		}
		fmt.Printf("created %s\n", deposit.GetName())
	case "send":
		if strings.TrimSpace(*beneficiary) == "" {
			log.Fatal("-beneficiary is required for send")
		}
		payment, err := client.SendPayment(ctx, &accountsv1.SendPaymentRequest{
			Parent: *parent, Beneficiary: *beneficiary, Amount: value,
			Reference: "Observability drill", RequestId: *requestID,
		})
		if err != nil {
			log.Fatalf("send payment: %v", err)
		}
		fmt.Printf("created %s\n", payment.GetName())
	default:
		log.Fatalf("unsupported -action %q", *action)
	}
	fmt.Printf("trace_id=%s\n", strings.Split(traceParent, "-")[1])
}

func parseMoney(amount, currency string) (*money.Money, error) {
	trimmedAmount := strings.TrimSpace(amount)
	if strings.HasPrefix(trimmedAmount, "-") {
		return nil, fmt.Errorf("amount must be positive")
	}
	parts := strings.Split(trimmedAmount, ".")
	if len(parts) > 2 || len(parts) == 0 {
		return nil, fmt.Errorf("invalid amount %q", amount)
	}
	units, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || units < 0 {
		return nil, fmt.Errorf("invalid amount %q", amount)
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	if len(fraction) > 2 {
		return nil, fmt.Errorf("amount supports at most two decimal places")
	}
	fraction += strings.Repeat("0", 2-len(fraction))
	cents, err := strconv.ParseInt(fraction, 10, 32)
	if err != nil || units == 0 && cents == 0 {
		return nil, fmt.Errorf("amount must be positive")
	}
	return &money.Money{CurrencyCode: strings.ToUpper(strings.TrimSpace(currency)), Units: units, Nanos: int32(cents) * 10_000_000}, nil
}

func newTraceParent() string {
	traceID := make([]byte, 16)
	spanID := make([]byte, 8)
	if _, err := rand.Read(traceID); err != nil {
		log.Fatalf("generate trace ID: %v", err)
	}
	if _, err := rand.Read(spanID); err != nil {
		log.Fatalf("generate span ID: %v", err)
	}
	return "00-" + hex.EncodeToString(traceID) + "-" + hex.EncodeToString(spanID) + "-01"
}
