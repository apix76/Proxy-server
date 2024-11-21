package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

type Route struct {
	Method        string
	PathAccess    string
	PathRedirect  string
	DominRedirect string
	Scheme        string
	PathFile      string
}

type ProxConf struct {
	YourIp       string
	HttpPort     string
	HttpsPort    string
	CertFilePath string
	KeyFilePath  string
	RedirectMap  map[string]Route
}

type Handler struct {
	route Route
}

func (r *Handler) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	log.Println(req.Host + " " + req.URL.String() + " " + req.Method)
	if r.route.Scheme != "" {
		req.URL.Scheme = r.route.Scheme
	} else {
		req.URL.Scheme = "https"
	}
	if r.route.DominRedirect != "" {
		req.URL.Host = r.route.DominRedirect
	}
	if r.route.PathRedirect != "" {
		req.URL.Path = r.route.PathRedirect
	}

	target, err := http.NewRequest(req.Method, req.URL.String(), req.Body)
	if err != nil {
		log.Fatal(err)
	}

	target.Header = req.Header
	log.Println(target.Host + " " + target.URL.String() + " " + target.Method)

	client := http.Client{}
	resp, err := client.Do(target)
	if err != nil {
		log.Println("proxy do error:", err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	defer resp.Body.Close()

	for k, vv := range resp.Header {
		res.Header()[k] = vv
	}
	res.WriteHeader(resp.StatusCode)

	io.Copy(res, resp.Body)
}

func (conf *ProxConf) RedirectToHttps(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, conf.YourIp+conf.HttpsPort+r.RequestURI, http.StatusMovedPermanently)
}

func (conf *ProxConf) HttpHttps() {
	mux := http.NewServeMux()
	conf.SetUpHandles(mux)
	go http.ListenAndServeTLS(conf.HttpsPort, conf.CertFilePath, conf.KeyFilePath, mux)
	http.ListenAndServe(conf.HttpPort, http.HandlerFunc(conf.RedirectToHttps))

}

func (conf *ProxConf) Http() {
	mux := http.NewServeMux()
	conf.SetUpHandles(mux)
	http.ListenAndServe(conf.HttpPort, mux)
}

func (conf *ProxConf) Check() error {
	if _, err := os.Stat(conf.CertFilePath); os.IsNotExist(err) {
		return err
	} else if _, err := os.Stat(conf.KeyFilePath); os.IsNotExist(err) {
		return err
	}
	return nil
}

func main() {
	var conf ProxConf
	fileCon, err := os.Open("config.cfg")
	if err != nil {
		fmt.Println("Couldn't open file or not exist, check him")
		panic("err")
	}

	defer fileCon.Close()
	err = json.NewDecoder(fileCon).Decode(&conf)
	if err != nil {
		fmt.Println("Couldn't decode file:", err)
		panic("err")
	}

	if conf.HttpPort != "" && conf.YourIp != "" {
		if conf.CertFilePath != "" && conf.KeyFilePath != "" && conf.HttpsPort != "" {
			err = conf.Check()
			if err != nil {
				fmt.Println("Invalid path or files are not exist")
				panic(err)
			}
			conf.HttpHttps()
		} else {
			conf.Http()
		}
	} else {
		fmt.Println("Config is empty")
	}

}

func (conf *ProxConf) SetUpHandles(mux *http.ServeMux) {
	for host, route := range conf.RedirectMap {
		var path string

		if route.Method != "" {
			path = route.Method + " " + host + "/" + route.PathAccess
		} else {
			path = host + "/" + route.PathRedirect
		}

		if route.PathFile != "" {
			mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
				http.ServeFile(w, r, route.PathFile)
			})
		} else {

			mux.Handle(path, &Handler{route})
		}
	}
}
