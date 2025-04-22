package auth

import (
	"log"
	"scanner/users"
)

// ValidatePassword return username, role, and true if the password is valid
func ValidatePassword(inputPassword string) (string, string, bool) {
	storedUsers, err := users.LoadUsers()
	if err != nil {
		log.Printf("unable to load passwords from json file: %s\n", err.Error())
		return "", "", false
	}

	for _, su := range storedUsers {
		if su.Password == inputPassword {
			return su.Username, su.Role, true
		}
	}

	return "", "", false
}
