package main

import (
	"fmt"
	"gol/out"
	"gol/surreal"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"

	"github.com/fxamacker/cbor/v2"
)

const (
	SETUP_QUERY_TEMPLATE = `
DEFINE DATABASE IF NOT EXISTS ⟨%s⟩; -- 0
USE DB ⟨%s⟩; -- 1
DEFINE TABLE IF NOT EXISTS catalog_counter SCHEMAFULL TYPE NORMAL; -- 2
DEFINE FIELD IF NOT EXISTS value ON catalog_counter TYPE int; -- 3
UPSERT ONLY catalog_counter:main SET value += 1 RETURN VALUE value - 1; -- 4`
	DEFINE_TABLE_QUERY_TEMPLATE = `
DEFINE TABLE ⟨%s⟩ SCHEMAFULL TYPE NORMAL; -- 0
DEFINE FIELD err  ON ⟨%s⟩ TYPE bool; -- 1
DEFINE FIELD msg  ON ⟨%s⟩ TYPE string; -- 2
DEFINE FIELD time ON ⟨%s⟩ TYPE option<datetime>; -- 3`
	CREATE_LOG_QUERY_TEMPLATE = `
CREATE ⟨%s⟩ SET err = $err, msg = $msg, time = $time RETURN NONE; -- 0`
)

func setupDb(db *surreal.Surreal, database string) (int, error) {
	q := fmt.Sprintf(SETUP_QUERY_TEMPLATE, database, database)
	r, err := db.Query(q, struct{}{})
	if err != nil {
		return 0, err
	}

	v, err := surreal.At[int](r, 4)
	if err != nil {
		return 0, err
	}

	return *v, nil
}

func defineTb(db *surreal.Surreal, table string) error {
	q := fmt.Sprintf(DEFINE_TABLE_QUERY_TEMPLATE, table, table, table, table)
	_, err := db.Query(q, struct{}{})

	return err
}

type createStdxxxVars struct {
	Err     bool      `cbor:"err"`
	Message string    `cbor:"msg"`
	Time    *cbor.Tag `cbor:"time"`
}

func createStdxxx(db *surreal.Surreal, table string, data *out.OutData, err bool) error {
	q := fmt.Sprintf(CREATE_LOG_QUERY_TEMPLATE, table)
	v := createStdxxxVars{
		Err:     err,
		Message: data.Message,
		Time:    data.Time,
	}
	_, e := db.Query(q, v)

	return e
}

func createStderr(db *surreal.Surreal, table string, data *out.OutData) error {
	return createStdxxx(db, table, data, true)
}

func createStdout(db *surreal.Surreal, table string, data *out.OutData) error {
	return createStdxxx(db, table, data, false)
}

type Table struct {
	Name string
}

func main() {
	log.SetFlags(0)

	args := os.Args
	if len(args) < 2 {
		log.Fatalln("Usage: gol <command> [args...]")
	}

	name, err := os.Hostname()
	if err != nil {
		log.Fatal(err)
	}

	host := os.Getenv("GOL_SURREAL_HOST")
	user := os.Getenv("GOL_SURREAL_USER")
	pass := os.Getenv("GOL_SURREAL_PASS")
	ns := os.Getenv("GOL_SURREAL_NS")

	db := surreal.New()
	tb := Table{}
	cmd := exec.Command(args[1], args[2:]...)
	stderr := out.New()
	stdout := out.New()
	defer func() {
		if cmd.ProcessState != nil {
			if !cmd.ProcessState.Exited() {
				if err := cmd.Process.Kill(); err != nil {
					_, err2 := stderr.Write([]byte(err.Error()))

					if err2 != nil {
						log.Println("kill error:", err)
						log.Println("write error:", err2)
					}
				}
			}
			if tb.Name != "" {
				if o := stderr.Consume(); len(o.Message) != 0 {
					if err := createStderr(db, tb.Name, o); err != nil {
						log.Println(err)
					}
				}
				if o := stdout.Consume(); len(o.Message) != 0 {
					if err := createStdout(db, tb.Name, o); err != nil {
						log.Println(err)
					}
				}
			}
			if !cmd.ProcessState.Success() {
				_, err := stderr.Write([]byte(cmd.ProcessState.String()))
				if err != nil {
					log.Println(err)
				} else if o := stderr.Consume(); len(o.Message) != 0 {
					if err := createStderr(db, tb.Name, o); err != nil {
						log.Println(err)
					}
				}
			} else {
				_, err := stdout.Write([]byte(cmd.ProcessState.String()))
				if err != nil {
					log.Println(err)
				} else if o := stdout.Consume(); len(o.Message) != 0 {
					if err := createStdout(db, tb.Name, o); err != nil {
						log.Println(err)
					}
				}
			}
		}
		if err := db.Close(); err != nil {
			log.Println(err)
		}
		log.Println("main: end")
		if cmd.ProcessState != nil {
			os.Exit(cmd.ProcessState.ExitCode())
		} else {
			os.Exit(1)
		}
	}()

	if err := db.Connect(host); err != nil {
		log.Fatal(err)
	}
	if err := db.UseNs(ns); err != nil {
		log.Fatal(err)
	}
	if err := db.Signin(user, pass); err != nil {
		log.Fatal(err)
	}
	i, err := setupDb(db, name)
	if err != nil {
		log.Fatal(err)
	}
	tb.Name = "_" + strconv.Itoa(i)
	if err := db.UseDb(name); err != nil {
		log.Fatal(err)
	}
	if err := defineTb(db, tb.Name); err != nil {
		log.Fatal(err)
	}

	osEnv := os.Environ()
	var cmdEnv []string
	for _, env := range osEnv {
		if !strings.HasPrefix(env, "GOL_") {
			cmdEnv = append(cmdEnv, env)
		}
	}

	cmd.Env = cmdEnv

	done := make(chan struct{})
	go func() {
		defer close(done)
		cmd.Stderr = stderr
		cmd.Stdout = stdout
		cmd.Run()
	}()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	for {
		select {
		case <-done:
			return
		case <-interrupt:
			return
		case <-db.CloseChan:
			log.Println(db.CloseErr)
			return
		case o := <-stderr.Ch:
			if err := createStderr(db, tb.Name, o); err != nil {
				log.Println("warn:", err)
			}
		case o := <-stdout.Ch:
			if err := createStdout(db, tb.Name, o); err != nil {
				log.Println("warn:", err)
			}
		}
	}
}
