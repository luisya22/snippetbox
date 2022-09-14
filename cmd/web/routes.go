package main

import (
	"net/http"
	"snippetbox.luismatosgarcia.com/ui"

	"github.com/julienschmidt/httprouter"
	"github.com/justinas/alice"
)

// Update the signature for the routes() method so that it returns a http.Handler
// instead of *http.ServerMux
func (app *application) routes() http.Handler {
	// Initialize the router
	router := httprouter.New()

	// Create a handler function which wraps our notFound() helper, and then assign it as the custom handler for
	// 404 Not Found responses. You can also set a custom handler for 405 Method Not Allowed by setting
	// router.MethodNotAllowed in the same way too
	router.NotFound = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app.notFound(w)
	})

	// Update the pattern for the route for the static files.
	//fileServer := http.FileServer(http.Dir("./ui/static"))
	//router.Handler(http.MethodGet, "/static/*filepath", http.StripPrefix("/static", fileServer)) //Normal method

	// With file embed
	// Take the ui.Files embedded filesystem and convert it to a http.FS type so that it satisfies the http.FileSystem
	// interface. We then pass that to the http.FileServer() function to create the file server handler.
	fileServer := http.FileServer(http.FS(ui.Files))

	// Our static files are contained in the "static" folder of the ui.Files embedded filesystem. So, for
	// example, our CSS stylesheet is located at "static/css/main.css". THis means that we no longer need to strip
	// the prefix from the request URL -- any requests that start with /static/ can just be passed directly
	// to the file server and the corresponding static file will be served (so long as it exists)
	router.Handler(http.MethodGet, "/static/*filepath", fileServer)

	// Add a new GET /ping route.
	router.HandlerFunc(http.MethodGet, "/ping", ping)

	// Create a new middleware chain containing the middleware specific to our dynamic application routes. For now,
	// this chain will only contain the LoadAndSave session middleware but we'll add more to it later
	dynamic := alice.New(app.sessionManager.LoadAndSave, noSurf, app.authenticate)

	// Unprotected application routes using the "dynamic" middleware chain
	// Create the routes using the appropriate methods, patterns and handlers.
	router.Handler(http.MethodGet, "/", dynamic.ThenFunc(app.home))
	router.Handler(http.MethodGet, "/snippet/view/:id", dynamic.ThenFunc(app.snippetView))
	router.Handler(http.MethodGet, "/user/signup", dynamic.ThenFunc(app.userSignup))
	router.Handler(http.MethodPost, "/user/signup", dynamic.ThenFunc(app.userSignupPost))
	router.Handler(http.MethodGet, "/user/login", dynamic.ThenFunc(app.userLogin))
	router.Handler(http.MethodPost, "/user/login", dynamic.ThenFunc(app.userLoginPost))

	// Protected (authenticated-only) application routes, using a new "protected" middleware chain
	// which includes the requireAuthentication middleware
	protected := dynamic.Append(app.requireAuthentication)

	router.Handler(http.MethodGet, "/snippet/create", protected.ThenFunc(app.snippetCreate))
	router.Handler(http.MethodPost, "/snippet/create", protected.ThenFunc(app.snippetCreatePost))
	router.Handler(http.MethodPost, "/user/logout", protected.ThenFunc(app.userLogoutPost))

	// Create a middleware chain containing our 'standard' middleware which will be used for every request our
	// application receives.
	standard := alice.New(app.recoverPanic, app.logRequest, secureHeaders)

	// Return the 'standard' middleware chain followed by the servermux
	return standard.Then(router)
}
