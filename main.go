package main

import (
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
)

var (
	Site           = flag.String("site", "", "the website to mirror, protocol with hostname only")
	CacheDirectory = flag.String("cache_directory", ".", "the directory to cache")
	Listen         = flag.String("listen", ":8000", "host port tuple to listen")
)

func main() {
	flag.Parse()

	if *Site == "" {
		log.Fatalln("A site must be provided.")
	}

	log.Printf("Mirror %s with cache directory %s\n", *Site, *CacheDirectory)

	m := Mirrorer{
		BaseURL:   *Site,
		Directory: *CacheDirectory,
	}

	http.Handle("/", m)

	log.Println("Listening", *Listen)
	err := http.ListenAndServe(*Listen, nil)
	if err != nil {
		log.Fatal(err)
	}

}

type Mirrorer struct {
	// The base URL to mirror
	BaseURL string
	// The directory to cache packages
	Directory string
}

// HasLocalFile determines if `uri` already exists.
func (m Mirrorer) HasLocalFile(uri string) bool {
	path := filepath.Join(m.Directory, uri)
	info, err := os.Stat(path)
	log.Println(info)
	return err == nil
}

// Implements http.Handler.
// The endpoint detects if a requested file exists in the local cache.
// If so, it serves the package from the local cache.
// Otherwise, it fetches the package, and saves the pakage to the local cache.
func (m Mirrorer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println(r.Method, r.RequestURI)
	hasFile := m.HasLocalFile(r.RequestURI)

	if hasFile {
		m.serve(w, r)
	} else {
		m.fetch(w, r)
	}
}

func (m Mirrorer) serve(w http.ResponseWriter, r *http.Request) {
	path := filepath.Join(m.Directory, r.RequestURI)
	f, err := os.Open(path)
	if err != nil {
		log.Println(err)
		return
	}
	defer f.Close()

	_, err = io.Copy(w, f)
	if err != nil {
		log.Println(err)
		return
	}
}

func (m Mirrorer) fetch(w http.ResponseWriter, r *http.Request) {
	req, err := http.NewRequest(r.Method, m.BaseURL+r.RequestURI, r.Body)
	if err != nil {
		log.Println(err)
		return
	}

	c := http.Client{}
	resp, err := c.Do(req)
	if err != nil {
		log.Println(err)
		return
	}
	defer resp.Body.Close()

	// NOTE(kfeng): if the package site is flaky, and returns a non-200 response.
	// do not cache the package.
	if resp.StatusCode != 200 {
		return
	}

	path := filepath.Join(m.Directory, r.RequestURI)
	log.Println("Writing to ", path)
	log.Println("Making Dir", filepath.Dir(path))
	cmd := exec.Command("mkdir", "-p", filepath.Dir(path))
	err = cmd.Run()
	if err != nil {
		log.Println(err)
		return
	}
	f, err := os.Create(path)
	if err != nil {
		log.Println(err)
		return
	}
	defer f.Close()

	teeReader := io.TeeReader(resp.Body, f)

	_, err = io.Copy(w, teeReader)
	if err != nil {
		log.Println(err)
		return
	}
}
