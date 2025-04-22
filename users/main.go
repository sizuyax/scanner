package users

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
)

const usersFile = "json/users.json"

var (
	usersMu sync.Mutex
)

type User struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	Password string `json:"password"`
}

func SaveUsers(inputUsers []User) error {
	usersList, err := LoadUsers()
	if err != nil {
		log.Printf("unable to load passwords from json file: %s\n", err.Error())
		return err
	}

	for _, iu := range inputUsers {

		for _, u := range usersList {
			if u.Username == iu.Username {
				return fmt.Errorf("user '%s' already exists", iu.Username)
			} else if u.Password == iu.Password {
				return fmt.Errorf("user '%s' with the same password already exists", u.Username)
			}
		}

		usersList = append(usersList, User{Username: iu.Username, Role: iu.Role, Password: iu.Password})
	}

	data, err := json.MarshalIndent(usersList, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(usersFile, data, 0644)
}

func ChangePassword(username, password string) error {
	usersList, err := LoadUsers()
	if err != nil {
		log.Printf("unable to load passwords from json file: %s\n", err.Error())
		return err
	}

	for i, pass := range usersList {
		if username == pass.Username {
			usersList[i].Password = password
			break
		}
	}

	data, err := json.MarshalIndent(usersList, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(usersFile, data, 0644)
}

func DeleteUser(username string) error {
	usersList, err := LoadUsers()
	if err != nil {
		log.Printf("unable to load users from json file: %s\n", err.Error())
		return err
	}

	found := false
	for i, user := range usersList {
		if user.Username == username {
			usersList = append(usersList[:i], usersList[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("user '%s' not found", username)
	}

	data, err := json.MarshalIndent(usersList, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(usersFile, data, 0644)
}

func LoadUsers() ([]User, error) {

	usersMu.Lock()
	defer usersMu.Unlock()

	data, err := os.ReadFile(usersFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []User{}, nil
		}
		return nil, err
	}

	if len(data) == 0 {
		return []User{}, nil
	}

	var storedUsers []User
	if err := json.Unmarshal(data, &storedUsers); err != nil {
		return nil, err
	}

	return storedUsers, nil
}
