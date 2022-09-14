package main

import (
	"crypto/tls"
	"database/sql"
	"flag"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/alexedwards/scs/mysqlstore"
	"github.com/alexedwards/scs/v2"
	"github.com/go-playground/form/v4"
	_ "github.com/go-sql-driver/mysql"
	"snippetbox.luismatosgarcia.com/internal/models"
)

// Define an application struct to hold the application-wide dependencies for the web application. For now we'll
// only include fields for the two custom loggers, but we'll only include fields for the two custom loggers, but
// we'll add more to it as the build progresses.
type application struct {
	errorLog       *log.Logger
	infoLog        *log.Logger
	snippets       models.SnippetModelInterface
	users          models.UserModelInterface
	templateCache  map[string]*template.Template // Add a templateCache field to the application struct
	formDecoder    *form.Decoder
	sessionManager *scs.SessionManager
}

func main() {
	// Define a new command-line flag with the name 'addr', a default value of ":4000" and some short help
	// text explaining what the flag controls. The value oc the flag will be stored in the addr variable at runtime
	addr := flag.String("addr", ":4000", "HTTP network address")

	// Define a new command-line flag for the MySQL DSN string
	dsn := flag.String("dsn", "root:local1234@/snippetbox?parseTime=true", "MySQL data source name")

	// Importantly, we use the flag.Parse() funcjtion to parse the command-line flag.
	// This reads in the command-line flag value and assigns it to the addr variable. You need to call this *before*
	// you use the addr variable otherwise it will always contain the default value of ":4000". If any errors are
	// encountered during parsing the application will be terminated.
	flag.Parse()

	// Use log.New() to create a logger for writing information messages. This takes three parameters:
	// teh destination to write the logs to (os.Stdout), a string prefix for message (Info followed by a tab),
	// and flags to indicate what additional information to include (local date and time). Note that the flags
	// are joined using the bitwise OR operator |.
	infoLog := log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime|log.Lshortfile)

	// Create a logger for writing error messages in the same way, but use stderr as the destination and use the
	// log.Lshortfile flag to include the relevant file name and line number
	errorLog := log.New(os.Stderr, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)

	// To keep the main function tidy I've put the code for creating a connection pool into the separate openDB()
	// function below. We pass openDB() the DSN from the command-line flag.
	db, err := openDB(*dsn)
	if err != nil {
		errorLog.Fatal(err)
	}

	// We also defer a call to db.Close(), so that the connection pool is closed before the main() function exists
	defer db.Close()

	// Initialize a new template cache...
	templateCache, err := newTemplateCache()
	if err != nil {
		errorLog.Fatal(err)
	}

	// Initialize a decoder instance
	formDecoder := form.NewDecoder()

	// Use the scs.New(0 function to initialize a new session manager. Then we configure it to use our MySQL database
	// as the session store, and set a lifetime of 12 hours (so that the sessions automatically expire 12 hours
	// after first being created).
	sessionManager := scs.New()
	sessionManager.Store = mysqlstore.New(db)
	sessionManager.Lifetime = 12 * time.Hour

	// Initialize a new instance of our application struct, containing the dependencies
	app := &application{
		errorLog:       errorLog,
		infoLog:        infoLog,
		snippets:       &models.SnippetModel{DB: db},
		users:          &models.UserModel{DB: db},
		templateCache:  templateCache,
		formDecoder:    formDecoder,
		sessionManager: sessionManager,
	}

	// Initialize a tls.Config struct to hold the non-default TLS settings we want the server to use. In this case
	// the only thing that we're changing is the curve preferences value, so that only elliptic curves with
	// assembly implementations are used.
	tlsConfig := &tls.Config{
		CurvePreferences: []tls.CurveID{tls.X25519, tls.CurveP256},
	}

	// Initialize a new http.Server struct. We set the Addr and Handler fields so that the server uses the same network
	// address and routes as before, and set the ErrorLog field so that the server now uses the custom errorLog logger
	// in the event of any problems.

	srv := &http.Server{
		Addr:      *addr,
		ErrorLog:  errorLog,
		Handler:   app.routes(),
		TLSConfig: tlsConfig,
		// Add Idle, Read and Write timeouts to the server.
		IdleTimeout:  time.Minute,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	infoLog.Printf("Starting server on %s", *addr)
	//err = srv.ListenAndServe()
	err = srv.ListenAndServeTLS("./tls/cert.pem", "./tls/key.pem")
	errorLog.Fatal(err)
}

func openDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	if err = db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}
