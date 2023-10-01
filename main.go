package main

import (
	"crypto/md5"
	"fmt"
	"github.com/netinternet/remoteaddr"
	"golang.org/x/mod/semver"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

var ListenAddr = os.Getenv("LISTEN_ADDR")
var FirmwarePath = os.Getenv("FIRMWARE_PATH")

func handler(writer http.ResponseWriter, request *http.Request) {
	if request.Header.Get("User-Agent") != "ESP8266-http-Update" {
		writer.WriteHeader(http.StatusBadRequest)
		return
	}
	remoteAddr, _ := remoteaddr.Parse().IP(request)
	remoteVersion := request.Header.Get("x-ESP8266-version")
	if remoteVersion == "" {
		writer.WriteHeader(http.StatusBadRequest)
		return
	}
	if path.Ext(request.URL.Path) != ".bin" {
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	remoteFilename := path.Base(request.URL.Path)
	firmwareName := remoteFilename[:len(remoteFilename)-len(filepath.Ext(remoteFilename))]
	firmwareDir := path.Join(FirmwarePath, firmwareName)
	log.Println(fmt.Sprintf(
		"Received request %s from %s firmware %s version %s",
		request.URL, remoteAddr, firmwareName, remoteVersion,
	))
	if info, err := os.Stat(firmwareDir); err != nil || !info.IsDir() {
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	localFiles, err := os.ReadDir(firmwareDir)
	if err != nil {
		log.Println("Firmware directory listing failed")
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	latest := ""
	for _, file := range localFiles {
		fileName := file.Name()
		if file.IsDir() || path.Ext(fileName) != ".bin" || !strings.HasPrefix(fileName, firmwareName) {
			continue
		}
		version := fileName[strings.LastIndexByte(fileName, '-')+1 : len(fileName)-len(filepath.Ext(fileName))]
		if semver.Compare(version, latest) > 0 {
			latest = version
		}
	}
	if semver.Compare(latest, remoteVersion) > 0 {
		filePath := path.Join(
			FirmwarePath, firmwareName, fmt.Sprintf("%s-%s.bin", firmwareName, latest),
		)
		fileBytes, err := ioutil.ReadFile(filePath)
		if err != nil {
			log.Println(fmt.Sprintf("Failed reading file: %s", filePath))
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		writer.Header().Set("Content-Type", "application/octet-stream")
		writer.Header().Set("Content-Length", strconv.Itoa(len(fileBytes)))
		writer.Header().Set("x-MD5", fmt.Sprintf("%x", md5.Sum(fileBytes)))
		writer.Header().Set(
			"Content-Disposition",
			fmt.Sprintf("attachment; filename=%s.bin", firmwareName),
		)
		//writer.WriteHeader(http.StatusOK)
		_, err = writer.Write(fileBytes)
		if err != nil {
			log.Println("Failed writing response body")
			return
		}
	} else {
		writer.WriteHeader(http.StatusNotModified)
	}
}

func main() {
	http.HandleFunc("/.ota/", handler)
	log.Fatal(http.ListenAndServe(ListenAddr, nil))
}
