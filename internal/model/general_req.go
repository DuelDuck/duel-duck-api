package model

import "gitlab.com/duel-duck/duel-duck-api/pkg/repository"

type OptsReq struct {
	Opts repository.Options `json:"opts" query:"opts"`
}
