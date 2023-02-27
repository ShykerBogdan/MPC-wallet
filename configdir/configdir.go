package configdir

// Based on https://github.com/ProtonMail/go-appdir

// Dirs requests application directories paths.
type Dirs interface {
	// Get the user-specific config directory.
	UserConfig() string
	// Get the user-specific cache directory.
	UserCache() string
	// Get the user-specific logs directory.
	UserLogs() string
	// Get the user-specific data directory.
	UserData() string
}

// New creates a new App with the provided name.
func New(name string) Dirs {
	return &dirs{name: name}
}

