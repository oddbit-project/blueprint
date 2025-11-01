package backend

type Authenticator interface {
	ValidateUser(userName string, secret string) (bool, error)
}
