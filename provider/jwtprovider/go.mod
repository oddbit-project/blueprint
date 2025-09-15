module github.com/oddbit-project/blueprint/provider/jwtprovider

go 1.23.0

require (
	github.com/golang-jwt/jwt/v5 v5.2.3
	github.com/google/uuid v1.6.0
	github.com/oddbit-project/blueprint v0.8.0
	github.com/rs/zerolog v1.34.0
	github.com/stretchr/testify v1.11.1
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	golang.org/x/sys v0.35.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/oddbit-project/blueprint => ../../
)