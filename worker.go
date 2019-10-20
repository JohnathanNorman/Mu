package main

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var testcases = 25
var interesting = 0
var crashes = 0

// Worker is a fuzzer worker, it does all the hard stuff
type Worker struct {
	config  Config
	workdir string
	asandir string
}

// NewWorker returns a worker
func NewWorker(config Config) Worker {
	return Worker{config, "", ""}
}

func (w Worker) setConfig(c Config, workdir string) Config {
	c.ProducerArgs = strings.Replace(c.ProducerArgs, "<outdir>", workdir, -1)
	c.ProducerArgs = strings.Replace(c.ProducerArgs, "<count>", strconv.Itoa(testcases), -1)
	c.ProducerArgs = strings.TrimSpace(c.ProducerArgs)
	c.Consumer = strings.TrimSpace(c.Consumer)

	if !strings.HasSuffix(c.ConsumerArgs, " ") {
		c.ConsumerArgs = c.ConsumerArgs + " "
	}

	return c
}

func (w Worker) start(id int) time.Duration {
	for {
		w.workdir = w.setAsan()
		w.config = w.setConfig(conf, w.workdir)
		w.asandir = filepath.Join(os.TempDir(), "asan")
		interesting = 0
		crashes = 0
		//TODO Measure producer/consumer time seperatly to optimize performance
		// when running multithreaded

		producerResult := w.runProducer()
		if producerResult.returnVal != 0 {
			log.Fatalf("producer returned %d error %s\n", producerResult.returnVal, producerResult.stdErr)
		}
		if producerResult.timedOut {
			log.Fatalf("Producer timed out\n")
		}

		w.runConsumer(id)
		// if we are here we completed the total count of testcases
		jobchan <- Job{id, testcases, crashes, interesting}

		if err := os.RemoveAll(w.workdir); err != nil {
			log.Fatalln("unable to delete workdir", err)
		}
	}
}

// sets asan environment variables and returns path the tempdir for asan crashes and returns
// the temp directory for asan files
func (w Worker) setAsan() string {
	asandir := filepath.Join(os.TempDir(), "asan")

	if _, err := os.Stat(asandir); os.IsNotExist(err) {
		merr := os.MkdirAll(asandir, os.ModePerm)
		if merr != nil {
			log.Fatalln("can't create temp asan dir error ->", merr)
		}
	}
	var asanenv string
	asanpath := filepath.Join(asandir, "asan-error")
	if w.config.Coverage {
		asanenv =
			fmt.Sprintf("log_path='%v':verbosity=0:coverage=1:coverage_dir='%v':exitcode=42:abort_on_error=false", asanpath, asandir)
	} else {
		asanenv = fmt.Sprintf("log_path='%v':verbosity=0:exitcode=42:abort_on_error=false", asanpath)
	}

	envErr := os.Setenv("ASAN_OPTIONS", asanenv)
	if envErr != nil {
		log.Fatalln("Can't set environment var error -> ", envErr)
	}

	workdir, err := ioutil.TempDir("", "workdir")
	if err != nil {
		log.Fatalln("Can't create tempdir ", err)
	}
	return workdir

}

// handles crash or interestingString found in program output
func (w Worker) handleCrash(config Config, tc string, rresult RunResult, isAsan bool) {
	// if crash dir isnt there make it
	if _, err := os.Stat(config.CrashDir); os.IsNotExist(err) {
		os.MkdirAll(config.CrashDir, os.ModePerm)
	}

	// create crash directory
	diruuid := uuid()
	tcdir := filepath.Join(config.CrashDir, diruuid)
	os.MkdirAll(tcdir, os.ModePerm)

	if _, err := copy(tc, filepath.Join(tcdir, filepath.Base(tc))); err != nil {
		log.Fatalln(err)
	}

	// write stdout to file ..
	// TODO this doesn't work when there is an ASAN Crash
	// invetigate why it catches stderr but not stdout
	stdoutfile := filepath.Join(tcdir, diruuid+".stdout")
	f, err := os.Create(stdoutfile)
	if err != nil {
		log.Fatalf("Can't create %s", stdoutfile)
	}
	f.WriteString(rresult.stdOut)
	// write std err to fifle
	stderrfile := filepath.Join(tcdir, diruuid+".stderr")
	f, err = os.Create(stderrfile)
	if err != nil {
		log.Fatalf("Can't create %s", stderrfile)
	}
	f.WriteString(rresult.stdErr)
	defer f.Close()

	if isAsan {
		// copy asan error log to crash dir
		// note error log  format will be <dir>/asan-error.52449
		asanerror := fmt.Sprintf("asan-error.%d", rresult.pid)

		if _, err := copy(filepath.Join(w.asandir, asanerror), filepath.Join(tcdir, asanerror)); err != nil {
			log.Fatalln("failed to copy asan crash ", err)
		}
		err := os.Remove(filepath.Join(w.asandir, asanerror))
		if err != nil {
			log.Fatalln("unable to remove asan file error->", err)
		}
	}
}

func (w Worker) handleCoverge(pid int) {
	// TODO support pulling coverage from multiple libs
	// filename + pid + .sancov
	sancovfn := fmt.Sprintf("%s.%d.sancov", filepath.Base(w.config.Consumer), pid)
	sancovFile := filepath.Join(w.asandir, sancovfn)
	// read coverage from file
	coverage, err := GetCoverage(sancovFile)
	if err != nil {
		log.Fatalln(err)
	}
	covchan <- coverage
	// remove so we don't read the same coverage again
	if err := os.Remove(sancovFile); err != nil {
		log.Fatalln("Can't remove sancov file->", err)
	}

	// asan will log the coverage output to an error file. we need to remove that
	// note error log  format will be <dir>/asan-error.52449
	asanerror := fmt.Sprintf("asan-error.%d", pid)

	// ignore errors, file might have been removed as part of a crash
	os.Remove(filepath.Join(w.asandir, asanerror))

}

func (w Worker) runProducer() RunResult {
	if w.config.DebugMode {
		fmt.Printf("[Producer]: %s %s\n", w.config.Producer, w.config.ProducerArgs)
	}

	cmdresult := w.run(w.config.Producer, w.config.ProducerArgs, time.Duration(w.config.ProducerTimeout))
	return cmdresult
}

func (w Worker) runConsumer(id int) {
	files, err := ioutil.ReadDir(w.workdir)
	if err != nil {
		log.Fatal(err)
	}

	if len(files) == 0 {
		log.Fatalln("[Consumer] dir ", w.workdir, " is empty!")
	}
	for _, f := range files {
		//check for asan-error files and skip
		if strings.Contains("asan-error", f.Name()) {
			fmt.Println("found asan-error file")
			continue
		}
		if w.config.DebugMode {
			fmt.Printf("[Consumer]: %s %s %s\n", w.config.Consumer, w.config.ConsumerArgs, filepath.Join(w.workdir, f.Name()))
		}

		finalArgs := w.config.ConsumerArgs + filepath.Join(w.workdir, f.Name())
		runResult := w.run(w.config.Consumer, finalArgs, time.Duration(w.config.ConsumerTimeout))

		// ASAN Crash
		if runResult.returnVal == 42 {
			w.handleCrash(w.config, filepath.Join(w.workdir, f.Name()), runResult, true)
			// just report crash and id. iterations counted elsewhere
			crashes = crashes + 1
		} else if w.checkOutput(runResult.stdErr, runResult.stdOut) {
			w.handleCrash(w.config, filepath.Join(w.workdir, f.Name()), runResult, false)
			interesting = interesting + 1
			if w.config.DebugMode {
				fmt.Println("Interesting output found")
			}
		} else if runResult.returnVal != 0 {
			// any other error type
			if w.config.DebugMode {
				fmt.Printf("Return code:%d\nStdErr:%v\nStdOut:%v", runResult.returnVal, runResult.stdErr, runResult.stdOut)
			}

		} else if runResult.timedOut {
			if w.config.DebugMode {
				fmt.Println("Timeout")
			}
		}

		if w.config.Coverage && runResult.timedOut == false {
			w.handleCoverge(runResult.pid)
		}
	}
}

// check command output for strings of interest
// interesting output is also stored with crashes for later review.
func (w Worker) checkOutput(stderr string, stdout string) bool {
	// lower to ignore case
	errLowered := strings.ToLower(stderr)
	outLowered := strings.ToLower(stdout)

	for _, intString := range w.config.InterestingStrings {
		intStringL := strings.ToLower(intString)
		if strings.Contains(errLowered, intStringL) {
			return true
		}
		if strings.Contains(outLowered, intStringL) {
			return true
		}
	}
	return false
}

// run is a basic command runner that returns  stderr, stdout
// and the return code from the program.
func (w Worker) run(command string, args string, timeout time.Duration) RunResult {
	//create context required for tiemout
	//fmt.Println(command, args)
	ctx, cancel := context.WithTimeout(context.Background(), timeout*time.Second)
	defer cancel()
	timedout := false
	// command setup
	realArgs := strings.Split(args, " ")
	cmd := exec.CommandContext(ctx, command, realArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	retval := 0

	if err := cmd.Start(); err != nil {
		log.Fatalf("cmd.Start: %v", err)
	}

	if err := cmd.Wait(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			if w.config.DebugMode {
				fmt.Println("Command timed out")
			}

			timedout = true
		}
		if exiterr, ok := err.(*exec.ExitError); ok {

			// The program has exited with an exit code != 0

			// This works on both Unix and Windows. Although package
			// syscall is generally platform dependent, WaitStatus is
			// defined for both Unix and Windows and in both cases has
			// an ExitStatus() method with the same signature.
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				retval = status.ExitStatus()
				//log.Printf("Exit Status: %d", status.ExitStatus())
			}

		} else {
			log.Fatalf("cmd.Wait: %v", err)
		}
	}
	outStr, errStr := string(stdout.Bytes()), string(stderr.Bytes())
	return RunResult{stdErr: errStr, stdOut: outStr, returnVal: retval, timedOut: timedout, pid: cmd.Process.Pid}
}
