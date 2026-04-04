package server

import (
	"context"
	"embed"
	"html/template"
	"io/fs"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

//go:embed web/assets/* web/templates/*
var webUIFiles embed.FS

var (
	webAssetsFS       = mustSubFS(webUIFiles, "web/assets")
	loginPageTemplate = template.Must(template.ParseFS(webUIFiles, "web/templates/login.html"))
	shellPageTemplate = template.Must(template.ParseFS(webUIFiles, "web/templates/shell.html"))
)

type webPageData struct {
	Title string
}

func registerWebUIRoutes(router chi.Router, deps Dependencies) {
	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		if hasValidSession(r.Context(), deps.Auth, r) {
			http.Redirect(w, r, "/app", http.StatusSeeOther)
			return
		}

		http.Redirect(w, r, "/app/login", http.StatusSeeOther)
	})

	router.Handle("/app/assets/*", http.StripPrefix("/app/assets/", http.FileServer(http.FS(webAssetsFS))))
	router.Get("/app/login", func(w http.ResponseWriter, r *http.Request) {
		if hasValidSession(r.Context(), deps.Auth, r) {
			http.Redirect(w, r, "/app", http.StatusSeeOther)
			return
		}

		renderPageTemplate(w, loginPageTemplate, webPageData{Title: "APOLLO Member Login"})
	})
	router.Group(func(app chi.Router) {
		app.Use(pageSessionMiddleware(deps.Auth))
		app.Get("/app", func(w http.ResponseWriter, r *http.Request) {
			renderPageTemplate(w, shellPageTemplate, webPageData{Title: "APOLLO Member Shell"})
		})
	})
}

func pageSessionMiddleware(authenticator Authenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(authenticator.SessionCookieName())
			if err != nil {
				http.Redirect(w, r, "/app/login", http.StatusSeeOther)
				return
			}

			principal, err := authenticator.AuthenticateSession(r.Context(), cookie.Value)
			if err != nil {
				http.Redirect(w, r, "/app/login", http.StatusSeeOther)
				return
			}

			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalContextKey, principal)))
		})
	}
}

func hasValidSession(ctx context.Context, authenticator Authenticator, r *http.Request) bool {
	cookie, err := r.Cookie(authenticator.SessionCookieName())
	if err != nil {
		return false
	}

	_, err = authenticator.AuthenticateSession(ctx, cookie.Value)
	return err == nil
}

func renderPageTemplate(w http.ResponseWriter, page *template.Template, data webPageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := page.Execute(w, data); err != nil {
		http.Error(w, "render page: "+err.Error(), http.StatusInternalServerError)
	}
}

func mustSubFS(source embed.FS, root string) fs.FS {
	result, err := fs.Sub(source, root)
	if err != nil {
		panic("sub fs " + strings.TrimSpace(root) + ": " + err.Error())
	}

	return result
}
