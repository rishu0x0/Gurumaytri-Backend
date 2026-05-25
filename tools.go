//go:build tools

// Package tools tracks dependencies used by subsequent plans.
// This file ensures go.mod retains jwt and crypto during go mod tidy,
// even before they are imported in business-logic packages.
package main

import (
	_ "github.com/golang-jwt/jwt/v5"
	_ "golang.org/x/crypto/bcrypt"
)
