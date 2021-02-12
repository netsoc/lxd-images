package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"log/syslog"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"syscall"

	"github.com/blang/semver/v4"
	"github.com/google/go-github/v33/github"
	"github.com/gorilla/mux"
)

var imageTagRegexp = regexp.MustCompile(`^(\S+)/v(.+)$`)
var (
	errNoVersion    = errors.New("no tagged version available")
	errStuffMissing = errors.New("release is missing image tarball / checksum")
)

var l = log.New(os.Stderr, "", log.LstdFlags)

var ghClient = github.NewClient(nil)

func listAllTags(ctx context.Context, owner, repo string) ([]*github.RepositoryTag, error) {
	opt := &github.ListOptions{
		PerPage: 50,
	}

	var allTags []*github.RepositoryTag
	for {
		tags, resp, err := ghClient.Repositories.ListTags(ctx, owner, repo, opt)
		if err != nil {
			return nil, err
		}

		allTags = append(allTags, tags...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return allTags, nil
}

func findImageVersion(ctx context.Context, owner, repo, name string) (semver.Version, error) {
	var latest semver.Version
	all, err := listAllTags(ctx, owner, repo)
	if err != nil {
		return latest, fmt.Errorf("failed to list repo tags: %w", err)
	}

	var allVersions []semver.Version
	for _, t := range all {
		m := imageTagRegexp.FindStringSubmatch(*t.Name)
		if len(m) == 0 || m[1] != name {
			continue
		}

		v, err := semver.Parse(m[2])
		if err != nil {
			continue
		}

		allVersions = append(allVersions, v)
	}
	if len(allVersions) == 0 {
		return latest, errNoVersion
	}

	for _, v := range allVersions {
		if v.GT(latest) {
			latest = v
		}
	}
	return latest, nil
}

func errToStatus(err error) int {
	var ghError *github.ErrorResponse
	switch {
	case errors.Is(err, errNoVersion):
		return http.StatusNotFound
	case errors.As(err, &ghError):
		return ghError.Response.StatusCode
	default:
		return http.StatusInternalServerError
	}
}
func errResponse(w http.ResponseWriter, err error) {
	l.Printf("Error: %v", err)
	w.WriteHeader(errToStatus(err))
	fmt.Fprintln(w, err)
}

func getTaggedImage(ctx context.Context, owner, repo, tag string) (string, string, error) {
	var url, sum string
	rel, _, err := ghClient.Repositories.GetReleaseByTag(ctx, owner, repo, tag)
	if err != nil {
		return url, sum, fmt.Errorf("failed to get release: %w", err)
	}

	for _, ass := range rel.Assets {
		switch *ass.Name {
		case "image.tar.xz":
			url = *ass.BrowserDownloadURL
		case "image.tar.xz.sha256":
			resp, err := http.Get(*ass.BrowserDownloadURL)
			if err != nil {
				return url, sum, fmt.Errorf("failed to download release sha256: %w", err)
			}

			defer resp.Body.Close()
			sumB := make([]byte, 64)
			if _, err := io.ReadFull(resp.Body, sumB); err != nil {
				return url, sum, fmt.Errorf("failed to read release sha256: %w", err)
			}

			sum = string(sumB)
		}
	}

	if url == "" || sum == "" {
		return url, sum, errStuffMissing
	}

	return url, sum, nil
}

func imageHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	var tag string
	vs, ok := vars["version"]
	if ok {
		tag = vars["image"] + "/v" + vs
	} else {
		l.Printf("Finding latest tag for %v/%v/%v", vars["owner"], vars["repo"], vars["image"])
		v, err := findImageVersion(r.Context(), vars["owner"], vars["repo"], vars["image"])
		if err != nil {
			errResponse(w, fmt.Errorf("failed to find latest version: %w", err))
			return
		}

		tag = fmt.Sprintf("%v/v%v", vars["image"], v)
	}

	l.Printf("Looking up release for %v/%v/%v", vars["owner"], vars["repo"], tag)
	url, sum, err := getTaggedImage(r.Context(), vars["owner"], vars["repo"], tag)
	if err != nil {
		errResponse(w, err)
		return
	}

	w.Header().Add("LXD-Image-Hash", sum)
	w.Header().Add("LXD-Image-URL", url)
	w.WriteHeader(http.StatusNoContent)
}

func main() {
	addr := flag.String("listen", ":8081", "listen address")
	certFile := flag.String("cert", "server.crt", "TLS certificate")
	keyFile := flag.String("key", "server.key", "TLS key")
	logToSyslog := flag.Bool("syslog", false, "write log messages to syslog")
	flag.Parse()

	var err error
	if *logToSyslog {
		l, err = syslog.NewLogger(syslog.LOG_ERR|syslog.LOG_LOCAL7, log.Lshortfile)
		if err != nil {
			log.Fatalf("Failed to create syslog logger: %v", err)
		}
	}

	r := mux.NewRouter()
	r.HandleFunc("/{owner}/{repo}/{image}", imageHandler).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc("/{owner}/{repo}/{image}/v{version}", imageHandler).Methods(http.MethodGet, http.MethodHead)
	server := http.Server{
		Addr:    *addr,
		Handler: r,
	}

	sigs := make(chan os.Signal)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		l.Fatal(server.ListenAndServeTLS(*certFile, *keyFile))
	}()

	<-sigs
	server.Close()
}
