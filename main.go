package main

import (
	initializers "cr-api/Initializers"
	"crm-api/auth"
	"crm-api/internal/graphql"
)

func init() {
	initializers.ConnectToDatabase()

	auth.InitGoogleStore()
}
func main() {
	graphql.Handler()
}
