package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/zitadel/oidc/v3/pkg/client/rp"
	"github.com/zitadel/oidc/v3/pkg/oidc"
)

func oidcSample() error {
	relyingParty, err := rp.NewRelyingPartyOIDC(
		context.Background(),
		"https://login.f110.dev",
		"306238040135762435",
		"ttbKIRvJilXRCpZmbjdlnNyb0bV6NZ6fGVrWlLkaxObLALaITWFhc0GofnzR7jId",
		"http://127.0.0.1:8082/callback",
		[]string{"openid", "profile", "email", "urn:zitadel:iam:org:projects:roles"},
	)
	if err != nil {
		return err
	}
	oauth2Config := relyingParty.OAuthConfig()

	http.HandleFunc("/auth", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, oauth2Config.AuthCodeURL(""), http.StatusFound)
	})
	http.HandleFunc("/callback", func(w http.ResponseWriter, req *http.Request) {
		token, err := oauth2Config.Exchange(context.Background(), req.URL.Query().Get("code"))
		if err != nil {
			log.Print(err)
			return
		}
		e := json.NewEncoder(os.Stdout)
		e.SetIndent("", "  ")
		e.Encode(token)
		rawIDToken := token.Extra("id_token").(string)
		log.Println(rawIDToken)
		idToken, err := rp.VerifyIDToken[*oidc.IDTokenClaims](context.Background(), rawIDToken, relyingParty.IDTokenVerifier())
		if err != nil {
			log.Print(err)
			return
		}

		e.Encode(idToken)
		json.NewEncoder(w).Encode(idToken)
	})
	http.ListenAndServe(":8082", nil)
	return nil
}

func main() {
	if err := oidcSample(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
