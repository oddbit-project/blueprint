module github.com/oddbit-project/blueprint/provider/httpserver

go 1.23.0

require (
	github.com/oddbit-project/blueprint v0.8.0
	github.com/gin-gonic/gin v1.10.1
	github.com/go-playground/validator/v10 v10.27.0
	github.com/golang-jwt/jwt/v5 v5.2.3
	github.com/prometheus/client_golang v1.20.5
)

replace github.com/oddbit-project/blueprint => ../../