package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
)

var covset []int64

func init() {
	covset = make([]int64, 0)
}

func is32BitHeader(data []byte) bool {
	var magicValue32 = []byte{0x32, 0xff, 0xff, 0xff, 0xff, 0xff, 0xbf, 0xc0}
	// haven't tested against 32 bit coverage reports yet
	if bytes.Equal(data, magicValue32) {
		return true
	}
	return false
}

func is64BitHeader(data []byte) bool {
	var magicValue64 = []byte{0x64, 0xff, 0xff, 0xff, 0xff, 0xff, 0xbf, 0xc0}
	if bytes.Equal(data, magicValue64) {
		return true
	}
	return false
}

func getFileSize(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil {
		log.Fatal("Error checking file stats", err)
	}
	return fi.Size()
}

// yes i know linear search is slow as hell.
// will spend this up later
func inCovset(val int64) bool {
	for i := 1; i <= len(covset); i++ {
		if val == covset[i-1] {
			return true
		}
	}
	return false
}

//GetCovStat returns info on the current code coverage
func GetCovStat() int {
	return len(covset)
}

// AddCoverage reads the code coverage from sancov file and
// if the PCS don't exist in the current coverage set adds them
func AddCoverage(fname string) {
	f, err := os.Open(fname)
	if err != nil {
		log.Fatal("Error while opening file", err)
	}

	for i := 0; ; i++ {
		data := make([]byte, 8)
		// ignoring errors for now
		bytesread, ferr := f.Read(data)
		if bytesread == 0 {
			break
		}
		if ferr == io.EOF {
			return
		}
		if ferr != nil {
			log.Fatalln(ferr)
		}

		if i == 0 {
			if !(is64BitHeader(data) || is32BitHeader(data)) {
				log.Fatalln("can't find header")
			} else {
				// don't add the header to covset
				continue
			}
		}
		var dint int64
		buf := bytes.NewReader(data)
		err := binary.Read(buf, binary.LittleEndian, &dint)
		if err != nil {
			fmt.Println("binary.Read failed:", err)
		}
		if !inCovset(dint) {
			covset = append(covset, dint)
		}
		fmt.Printf("value: %x dec : %d\n", data, dint)
	}
}

// GetCoverage simply reads the coverage from input file
// returns it as an array. doesn't add to local covset
func GetCoverage(fname string) ([]int64, error) {
	f, err := os.Open(fname)
	if err != nil {
		return nil, err
	}

	mycov := make([]int64, 0)
	for i := 0; ; i++ {
		data := make([]byte, 8)
		bytesread, ferr := f.Read(data)
		if ferr == io.EOF {
			return mycov, nil
		}
		if ferr != nil {
			return nil, ferr
		}
		if bytesread == 0 {
			break
		}

		if i == 0 {
			if !(is64BitHeader(data) || is32BitHeader(data)) {
				return nil, errors.New("can't find header")
			}
			// don't add the header to covset
			continue
		}
		var dint int64
		buf := bytes.NewReader(data)
		err := binary.Read(buf, binary.LittleEndian, &dint)
		if err != nil {
			return nil, err
		}
		mycov = append(mycov, dint)
	}
	return mycov, nil
}
