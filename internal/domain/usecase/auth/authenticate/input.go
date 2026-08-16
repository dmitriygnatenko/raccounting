package authenticate

// Input is what AuthenticateSession needs to resolve a session token into its user.
type Input struct {
	Token string
}
