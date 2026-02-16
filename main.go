package main

import (
	"os"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook/cmd"
	"cert-manager-volcdns-webhook/volcengine"
)

func main() {
	groupName := os.Getenv("GROUP_NAME")
	if groupName == "" {
		panic("GROUP_NAME must be specified")
	}

	// This will register our custom DNS provider with the webhook server.
	// Library authors can add other providers here.
	cmd.RunWebhookServer(groupName,
		volcengine.NewSolver(),
	)
}
