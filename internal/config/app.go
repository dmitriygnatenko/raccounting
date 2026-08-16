package config

// Application defaults, used whenever the corresponding env var isn't set. The demo credentials only
// ever reach a freshly seeded database (see seedDemoUser), never an existing account.
const (
	defaultPort         = "8080"
	defaultDemoUsername = "user"
	defaultDemoPassword = "pass"
)

// AppConfig collects how the server itself runs: where to listen, whether cookies are HTTPS-only,
// and the credentials of the demo user seeded on first run. Storage lives in DBConfig and logging in
// LogConfig, each loaded on its own.
type AppConfig struct {
	Port         string
	CookieSecure bool
	DemoUsername string
	DemoPassword string
}

// LoadApp builds the server configuration from the environment, rejecting anything malformed as it
// goes. Every setting here is optional and falls back to a default — the app is meant to run with an
// empty environment — but a value that is set and malformed is an error rather than a silent
// fallback, so a typo like COOKIE_SECURE=yes fails at startup instead of quietly serving cookies
// without the Secure flag.
func LoadApp() (AppConfig, error) {
	cookieSecure, err := boolEnv("COOKIE_SECURE", false)
	if err != nil {
		return AppConfig{}, err
	}

	port, err := portEnv("PORT", defaultPort)
	if err != nil {
		return AppConfig{}, err
	}

	return AppConfig{
		Port:         port,
		CookieSecure: cookieSecure,
		DemoUsername: stringEnv("DEMO_USERNAME", defaultDemoUsername),
		DemoPassword: stringEnv("DEMO_PASSWORD", defaultDemoPassword),
	}, nil
}
