package main

import (
	"flag"
	"fmt"
	"github.com/oddbit-project/blueprint/provider/htpasswd"
	"os"
	"syscall"

	"golang.org/x/term"
)

var (
	createFile  = flag.Bool("c", false, "Create a new file")
	deleteUser  = flag.Bool("D", false, "Delete the specified user")
	verifyUser  = flag.Bool("v", false, "Verify password for the specified user")
	batchMode   = flag.Bool("b", false, "Use batch mode (password on command line)")
	algorithm   = flag.String("B", "bcrypt", "Force the hash algorithm (bcrypt, apr1, sha, sha256, sha512, crypt, plain)")
	showHelp    = flag.Bool("h", false, "Show this help message")
	showVersion = flag.Bool("version", false, "Show version information")
)

const version = "1.0.0"

type HtpasswdFile struct {
	filename string
	create   bool
	c        *htpasswd.Container
}

func main() {
	flag.Usage = usage
	flag.Parse()

	if *showHelp {
		usage()
		os.Exit(0)
	}

	if *showVersion {
		fmt.Printf("htpasswd version %s\n", version)
		os.Exit(0)
	}

	args := flag.Args()
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "Error: insufficient arguments\n\n")
		usage()
		os.Exit(1)
	}

	passwordFile := args[0]
	username := args[1]
	var password string

	// Get password from command line if in batch mode
	if *batchMode && len(args) >= 3 {
		password = args[2]
	}

	htFile, err := NewHtpasswdFile(*createFile, passwordFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	// Handle different operations
	switch {
	case *deleteUser:
		err = htFile.DeleteUser(username)
	case *verifyUser:
		err = htFile.VerifyUser(username)
	default:
		err = htFile.AddUser(username, password)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `htpasswd - Manage user files for basic authentication

USAGE:
    htpasswd [OPTIONS] passwordfile username [password]

DESCRIPTION:
    htpasswd is used to create and update the flat-files used to store
    usernames and password for basic authentication of HTTP users.

OPTIONS:
    -c           Create a new file
    -D           Delete the specified user
    -v           Verify password for the specified user
    -b           Use batch mode (password on command line)
    -B algorithm Force the hash algorithm (bcrypt, apr1, sha, sha256, sha512, crypt, plain)
    -h           Show this help message
    -version     Show version information

EXAMPLES:
    htpasswd -c /etc/apache2/.htpasswd username
    htpasswd /etc/apache2/.htpasswd username
    htpasswd -D /etc/apache2/.htpasswd username
    htpasswd -v /etc/apache2/.htpasswd username
    htpasswd -b -B sha256 /etc/apache2/.htpasswd username password

`)
}

func NewHtpasswdFile(create bool, filename string) (*HtpasswdFile, error) {
	var err error
	var container *htpasswd.Container

	// if not creating new, attempt to load existing file
	if !create {
		container, err = htpasswd.NewFromFile(filename)
		if err != nil {
			return nil, fmt.Errorf("failed to load htpasswd file '%s': %w", filename, err)
		}
	} else {
		container = htpasswd.NewContainer()
	}
	return &HtpasswdFile{
		filename: filename,
		create:   create,
		c:        container,
	}, nil
}

func (f *HtpasswdFile) Save() error {
	file, err := os.OpenFile(f.filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", f.filename, err)
	}
	defer file.Close()

	return f.c.Write(file)
}

func (f *HtpasswdFile) DeleteUser(username string) error {
	if !f.c.UserExists(username) {
		return fmt.Errorf("user %s not found", username)
	}

	if err := f.c.DeleteUser(username); err != nil {
		return fmt.Errorf("failed to delete user '%s': %w", username, err)
	}

	if err := f.Save(); err != nil {
		return fmt.Errorf("failed to save file: %w", err)
	}

	fmt.Printf("Deleting password for user %s\n", username)
	return nil
}

func (f *HtpasswdFile) AddUser(username, password string) error {
	// Check if user exists when not creating file
	if !f.create && f.c.UserExists(username) {
		fmt.Printf("Changing password for user %s\n", username)
	} else {
		fmt.Printf("Adding password for user %s\n", username)
	}

	// Get password if not provided
	if password == "" {
		var err error
		password, err = readPassword("New password: ")
		if err != nil {
			return fmt.Errorf("failed to read password: %w", err)
		}

		confirmPassword, err := readPassword("Re-type new password: ")
		if err != nil {
			return fmt.Errorf("failed to read confirmation password: %w", err)
		}

		if password != confirmPassword {
			return fmt.Errorf("passwords do not match")
		}
	}

	// Get hash type from algorithm flag
	hashType, err := parseAlgorithm(*algorithm)
	if err != nil {
		return fmt.Errorf("invalid algorithm: %w", err)
	}

	// Add or update user
	if err := f.c.AddUserWithHash(username, password, hashType); err != nil {
		return fmt.Errorf("failed to add user: %w", err)
	}

	// Save file
	if err := f.Save(); err != nil {
		return fmt.Errorf("failed to save file: %w", err)
	}

	return nil
}

func (f *HtpasswdFile) VerifyUser(username string) error {
	if !f.c.UserExists(username) {
		return fmt.Errorf("user %s not found", username)
	}

	password, err := readPassword("Password: ")
	if err != nil {
		return fmt.Errorf("failed to read password: %w", err)
	}

	valid, err := f.c.VerifyUser(username, password)
	if err != nil {
		return fmt.Errorf("failed to verify user: %w", err)
	}

	if valid {
		fmt.Printf("Password for user %s correct.\n", username)
	} else {
		fmt.Printf("Password verification failed.\n")
		os.Exit(1)
	}

	return nil
}

func readPassword(prompt string) (string, error) {
	fmt.Print(prompt)

	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", err
	}

	fmt.Println() // Add newline after password input
	return string(bytePassword), nil
}

func parseAlgorithm(alg string) (htpasswd.HashType, error) {
	switch alg {
	case "bcrypt":
		return htpasswd.HashTypeBcrypt, nil
	case "apr1":
		return htpasswd.HashTypeApacheMD5, nil
	case "sha":
		return htpasswd.HashTypeSHA1, nil
	case "sha256":
		return htpasswd.HashTypeSHA256, nil
	case "sha512":
		return htpasswd.HashTypeSHA512, nil
	case "crypt":
		return htpasswd.HashTypeCrypt, nil
	case "plain":
		return htpasswd.HashTypePlain, nil
	default:
		return htpasswd.HashTypeBcrypt, fmt.Errorf("unsupported algorithm: %s", alg)
	}
}
