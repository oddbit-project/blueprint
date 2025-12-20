# File Logging

The Blueprint framework includes support for logging to files in addition to console output. This allows for persistent logs that can be reviewed and analyzed after the application has exited.

## Basic Configuration

To enable file logging in your application, configure the logger with the appropriate file output settings:

```go
// Create a logger configuration
logConfig := log.NewDefaultConfig()

// Enable file logging with a specific path
log.EnableFileOutput(logConfig, "/path/to/logs/application.log")

// Configure the logger
if err := log.Configure(logConfig); err != nil {
    // Handle configuration error
}
```

## File Output Options

The logger provides several configuration options for file output:

| Option | Description | Default |
|--------|-------------|---------|
| `FileOutput` | Whether to enable file logging | `false` |
| `FilePath` | Path to the log file | `application.log` |
| `FileFormat` | Output format, either "json" or "console" | `json` |
| `FileAppend` | Whether to append to an existing file | `true` |
| `FilePermissions` | File permissions | `0644` |

## Helper Functions

The logger provides helper functions to simplify configuration:

### EnableFileOutput

Enables file logging with the specified file path:

```go
config := log.NewDefaultConfig()
config = log.EnableFileOutput(config, "/path/to/logs/app.log")
```

### SetFileFormat

Sets the output format for file logging:

```go
// For human-readable console-like format
config = log.SetFileFormat(config, "console")

// For structured JSON format (better for log processing)
config = log.SetFileFormat(config, "json")
```

### DisableFileAppend

By default, logs are appended to existing files. Use this function to overwrite existing files instead:

```go
config = log.DisableFileAppend(config)
```

## Log Rotation

The Blueprint logger includes built-in log rotation support via the lumberjack library. Configure rotation using these Config fields:

| Option | Description | Default |
|--------|-------------|---------|
| `FileRotation` | Enable log rotation | `false` |
| `MaxSizeMb` | Maximum log file size in MB before rotation | `100` |
| `MaxBackups` | Maximum number of old log files to retain | `3` |
| `MaxAgeDays` | Maximum age in days to keep old log files | `28` |
| `Compress` | Compress rotated log files | `false` |

### Rotation Configuration Example

```go
config := log.NewDefaultConfig()
config = log.EnableFileOutput(config, "/var/log/myapp/app.log")

// Enable rotation
config.FileRotation = true
config.MaxSizeMb = 50       // Rotate when file reaches 50MB
config.MaxBackups = 5       // Keep 5 old files
config.MaxAgeDays = 30      // Delete files older than 30 days
config.Compress = true      // Compress old files with gzip

if err := log.Configure(config); err != nil {
    panic(err)
}
```

### Alternative Approaches

If you prefer external rotation, you can also:

1. Create a new log file with a timestamp for each application run
2. Use an external log rotation solution like logrotate
3. Integrate a third-party log rotation library

## Cleanup

The logger manages open file handles internally. However, for clean shutdowns, you can explicitly close log files:

```go
// Before application exit
log.CloseLogFiles()
```

## Console and File Simultaneously

When file logging is enabled, log messages are sent to both the console and the file by default. This makes it easy to see logs in real-time while also preserving them for later analysis.

## Example

Here's a complete example of setting up file logging:

```go
package main

import (
    "github.com/oddbit-project/blueprint/log"
    "os"
    "path/filepath"
    "time"
)

func main() {
    // Create logs directory
    os.MkdirAll("logs", 0755)
    
    // Timestamp-based filename
    timestamp := time.Now().Format("2006-01-02_15-04-05")
    logFilePath := filepath.Join("logs", "app_" + timestamp + ".log")
    
    // Configure logger
    config := log.NewDefaultConfig()
    config = log.EnableFileOutput(config, logFilePath)
    config = log.SetFileFormat(config, "json")  // Structured JSON format
    
    if err := log.Configure(config); err != nil {
        panic(err)
    }
    
    // Create a logger
    logger := log.New("myapp")
    
    // Log some messages
    logger.Info("Application started")
    
    // Log with structured fields
    logger.Info("User authenticated", log.KV{
        "user_id": 12345,
        "role":    "admin",
    })
    
    // Clean up
    log.CloseLogFiles()
}
```

## Best Practices

1. **Use structured logging**: JSON format is better for log processing and analysis tools
2. **Include contextual information**: Use key-value pairs to provide context
3. **Create log directories**: Ensure log directories exist before configuring the logger
4. **Use timestamp-based filenames**: For applications without long-running sessions
5. **Set appropriate log levels**: Use Debug for development and Info for production
6. **Close log files**: Call `CloseLogFiles()` during application shutdown