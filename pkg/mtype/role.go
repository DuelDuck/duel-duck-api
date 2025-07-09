package mtype

const (
	UserRole  Role = 0
	AdminRole Role = 1
)

type Role uint8

func (r Role) IsValid() bool {
	switch r {
	case UserRole, AdminRole:
		return true
	}

	return false
}

func (r Role) Admin() bool {
	return r == AdminRole
}
