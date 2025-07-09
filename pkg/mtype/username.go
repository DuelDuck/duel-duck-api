package mtype

import "regexp"

var usernameRegexp = regexp.MustCompile(`^[a-zA-Z0-9\._\-]{2,44}$`)

type Username string

func NewUsername(username string) (Username, bool) {
	if !usernameRegexp.MatchString(username) {
		return "", false
	}

	return Username(username), true
}

func (u Username) String() string {
	return string(u)
}

func (u Username) Valid() bool {
	return usernameRegexp.MatchString(u.String())
}
