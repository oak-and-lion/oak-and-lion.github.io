package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

var debug = false
var filterFile = ""
var defaultPage = "index.html"
var handlers []filterHandler
var corsList []corsAllowed

type filterHandler struct {
	extension, filterH, filterArg, filterArg2 string
}

type corsAllowed struct {
	origin string
}

func GetIPAddress(desiredIP string) string {
	if desiredIP == "_" {
		return "localhost"
	}

	var result = ""
	name, err := os.Hostname()
	fmt.Printf("Server name: " + name + "\n")
	if err != nil {
		fmt.Printf("Oops: %v\n", err)
		return ""
	}

	addrs, err := net.LookupHost(name)
	if err != nil {
		fmt.Printf("Oops: %v\n", err)
		return ""
	}

	for _, a := range addrs {
		fmt.Printf("Available IP: " + a + "\n")
		if desiredIP == "" || a == desiredIP {
			result = a
			break
		}
	}

	if result == "127.0.1.1" || result == "" {
		fmt.Printf("defaulting to desired IP\n")
		result = desiredIP
	}
	return result
}

func Readln(r *bufio.Reader) (string, error) {
	var (
		isPrefix bool  = true
		err      error = nil
		line, ln []byte
	)
	for isPrefix && err == nil {
		line, isPrefix, err = r.ReadLine()
		ln = append(ln, line...)
	}
	return string(ln), err
}

func checkForFilters(url string) int {
	var result = -1

	for index, element := range handlers {
		if strings.Contains(url, element.extension) {
			result = index
			break
		}
	}

	return result
}

func handler(w http.ResponseWriter, r *http.Request) {
	foundIndex := 0
	foundOrigin := "*"
	origin := strings.ToLower(r.Header.Get("Origin"))
	if origin != "" {
		if debug {
			fmt.Println("Requested CORS Origin: " + origin)
		}
		foundIndex = -1
		for index, element := range corsList {
			if origin == element.origin {
				foundIndex = index
				foundOrigin = corsList[foundIndex].origin
				break
			}
		}
	}
	if foundIndex < 0 {
		w.Header().Add("Content-type", "text/plain")
		w.WriteHeader(500)
		fmt.Fprint(w, "Invalid CORS request, origin: "+foundOrigin)
		if debug {
			fmt.Println("CORS origin not found on allowed list: " + origin)
		}
	} else {
		w.Header().Add("Access-Control-Allow-Origin", foundOrigin)
		handleRequest(w, r)
	}
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	url := strings.Split(strings.Replace(r.URL.String(), "/", "", 1), "?")[0]
	if debug {
		if url == "" {
			fmt.Println("request made for default page (" + defaultPage + ")")
		} else {
			fmt.Println("request made for " + url)
		}
	}
	if url == "" {
		stringHandler(w, defaultPage)
	} else {
		if url == "panic.go" {
			fmt.Println("\n\nPANIC COMMAND SENT REMOTELY!\nFatal error: aborting")
			fmt.Println("\n")
			fmt.Println(r)
			os.Exit(9999)
		} else if strings.Contains(url, ".woff") || strings.Contains(url, ".woff2") || strings.Contains(url, ".ttf") || strings.Contains(url, ".gif") || strings.Contains(url, ".png") || strings.Contains(url, ".jpg") || strings.Contains(url, ".ico") || strings.Contains(url, ".mp3") || strings.Contains(url, ".ogg") || strings.Contains(url, ".pdf") || strings.Contains(url, ".mpg") || strings.Contains(url, ".mpeg") || strings.Contains(url, ".mp4") || strings.Contains(url, ".m4v") || strings.Contains(url, ".avi") || strings.Contains(url, ".mov") {
			if debug {
				fmt.Println("request made for " + url + " is for binary data")
			}
			binaryHandler(w, url)
		} else if strings.Contains(url, ".html") || strings.Contains(url, ".htm") || strings.Contains(url, ".txt") || strings.Contains(url, ".css") || strings.Contains(url, ".js") || strings.Contains(url, ".csv") || strings.Contains(url, ".xml") {
			if debug {
				fmt.Println("request made for " + url + " is for string data")
			}
			stringHandler(w, url)
		} else {
			index := checkForFilters(url)
			if index > -1 {
				cmd := exec.Command("cmd")
				if handlers[index].filterArg2 == "?" {
					cmd = exec.Command(handlers[index].filterH, handlers[index].filterArg)

				} else {
					cmd = exec.Command(handlers[index].filterH, handlers[index].filterArg, handlers[index].filterArg2)

				}
				body, _ := ioutil.ReadAll(r.Body)
				cmd.Stdin = strings.NewReader(r.URL.String() + "^~~^" + string(body[:]))
				var out bytes.Buffer
				cmd.Stdout = &out
				err := cmd.Start()
				err = cmd.Wait()

				if err == nil {
					var checker = out.Next(1)[0]
					if checker == '_' {
						var t = out.Next(1)[0]
						switch t {
						case '1':
							w.Header().Add("Content-type", "text/html")
						case '2':
							w.Header().Add("Content-type", "application/javascript")
						case '3':
							w.Header().Add("Content-type", "text/css")
						case '4':
							w.Header().Add("Content-type", "text/xml")
						case '5':
							w.Header().Add("Content-type", "application/json")
						default:
							w.Header().Add("Content-type", "text/plain")
						}
						var status = string(out.Next(3))
						var responseStatus, ierr = strconv.Atoi(status)

						if ierr == nil {
							var checkerEnd = out.Next(1)[0]
							if checkerEnd == '_' {
								w.WriteHeader(responseStatus)
								out.WriteTo(w)
							} else {
								w.WriteHeader(500)
								fmt.Fprintf(w, "Filter Returned Invalid data format.\nFormat is _<type><statusCode>_\nex. _1200_")
								fmt.Fprint(w, "Returned: ")
								fmt.Fprint(w, checker)
								fmt.Fprint(w, t)
								fmt.Fprint(w, status)
								fmt.Fprint(w, checkerEnd)
								out.WriteTo(w)
							}
						} else {
							w.WriteHeader(500)
							fmt.Fprintf(w, "Filter Returned Invalid data format.\nFormat is _<type><statusCode>_\nex. _1200_")
							fmt.Fprint(w, "Returned: ")
							fmt.Fprint(w, checker)
							fmt.Fprint(w, t)
							fmt.Fprint(w, status)
							out.WriteTo(w)
						}
					} else {
						w.Header().Add("Content-type", "text/plain")
						w.WriteHeader(500)
						fmt.Fprintf(w, "Filter Returned Invalid data format.\nFormat is _<type><statusCode>_\nex. _1200_")
						fmt.Fprint(w, "Returned: ")
						fmt.Fprint(w, checker)
						out.WriteTo(w)
					}
				} else {
					if debug {
						fmt.Println(err)
					}
					w.Header().Add("Content-type", "text/plain")
					w.WriteHeader(500)
					fmt.Fprintf(w, "Filter Error\n"+handlers[index].filterH+" "+handlers[index].filterArg+"\n")
					fmt.Fprintf(w, err.Error())
				}
			} else {
				w.Header().Add("Content-type", "text/plain")
				w.WriteHeader(500)
				fmt.Fprintf(w, "Page type not valid")
			}
		}
	}
}

func binaryHandler(w http.ResponseWriter, url string) {
	b, err := ioutil.ReadFile(url)
	if err == nil {
		if strings.Contains(url, ".gif") {
			w.Header().Add("Content-type", "image/gif")
		} else if strings.Contains(url, ".png") {
			w.Header().Add("Content-type", "image/png")
		} else if strings.Contains(url, ".jpg") {
			w.Header().Add("Content-type", "image/jpeg")
		} else if strings.Contains(url, ".ico") {
			w.Header().Add("Content-type", "image/vnd.microsoft.icon")
		} else if strings.Contains(url, ".mp3") {
			w.Header().Add("Content-type", "audio/mpeg")
			w.Header().Add("Transfer-Encoding", "chunked")
		} else if strings.Contains(url, ".mpg") || strings.Contains(url, ".mpeg") || strings.Contains(url, ".mp4") || strings.Contains(url, ".m4v") || strings.Contains(url, ".avi") {
			w.Header().Add("Content-type", "video/mpeg")
			w.Header().Add("Transfer-Encoding", "chunked")
		} else if strings.Contains(url, ".mov") {
			w.Header().Add("Content-type", "video/quicktime")
			w.Header().Add("Transfer-Encoding", "chunked")
		} else if strings.Contains(url, ".ogg") {
			w.Header().Add("Content-type", "audio/ogg")
			w.Header().Add("Transfer-Encoding", "chunked")
		} else if strings.Contains(url, ".pdf") {
			w.Header().Add("Content-type", "application/pdf")
		}
		w.Header().Add("Size-Baby", strconv.Itoa(binary.Size(b)))
		w.Header().Add("Content-Length", strconv.Itoa(binary.Size(b)))
		w.WriteHeader(200)
		binary.Write(w, binary.BigEndian, &b)
		if debug {
			fmt.Printf("response headers for " + url + " ")
			fmt.Println(w.Header())
			fmt.Println("end request for " + url)
		}
	} else {
		w.WriteHeader(404)
		if debug {
			fmt.Println(url + " was not found")
			fmt.Println("end request for " + url)
		}
	}
}

func stringHandler(w http.ResponseWriter, url string) {
	b, err := ioutil.ReadFile(url)
	if err == nil {
		if strings.Contains(url, ".txt") {
			w.Header().Add("Content-Type", "text/plain")
		} else if strings.Contains(url, ".js") {
			w.Header().Add("Content-Type", "application/javascript")
		} else if strings.Contains(url, ".css") {
			w.Header().Add("Content-Type", "text/css")
		} else if strings.Contains(url, ".csv") {
			w.Header().Add("Content-Type", "text/csv")
		} else if strings.Contains(url, ".xml") {
			w.Header().Add("Content-Type", "text/xml")
		} else {
			w.Header().Add("Content-Type", "text/html")
		}
		w.Header().Add("Content-Length", strconv.Itoa(binary.Size(b)))
		w.WriteHeader(200)
		binary.Write(w, binary.BigEndian, &b)
		if debug {
			fmt.Printf("response headers for " + url + " ")
			fmt.Println(w.Header())
			fmt.Println("end request for " + url)
		}
	} else {
		w.WriteHeader(404)
		fmt.Fprintf(w, "404 Page Not Found /"+url)
		if debug {
			fmt.Println(url + " was not found")
			fmt.Println("end request for " + url)
		}
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: webServer port [configFile] [d] (for debug mode)")
		fmt.Println("configFile is optional, and describes external filters, etc. for handling http requests")
		fmt.Println("if you want to run debug mode without specifying a configFile,")
		fmt.Println("use the word 'nil' for the argument")
		fmt.Println("ex: webServer 8100 nil d")
	} else {
		if len(os.Args) >= 4 {
			if strings.Trim(os.Args[3], " ") == "d" {
				debug = true
			}
		}
		s := strings.Trim(os.Args[1], " ")
		var desiredIP = "127.0.0.1"
		if len(os.Args) >= 3 && os.Args[2] != "nil" {
			filterFile = os.Args[2]
			f, err := os.Open(filterFile)
			fmt.Println("=================\nconfig file: " + filterFile)
			if err != nil {
				fmt.Printf("error opening file: %v\n", err)
				os.Exit(1)
			}
			r := bufio.NewReader(f)
			s, e := Readln(r)
			fmt.Println("External http filters")
			for e == nil {
				var line = strings.Split(s, " ")
				if line[0] == "filter" {
					filt := filterHandler{line[1], line[2], strings.Replace(line[3], "_", "", -1), strings.Replace(line[4], "_", "", -1)}
					handlers = append(handlers, filt)
					fmt.Printf("\t")
					fmt.Println(filt)
				} else if line[0] == "ip" {
					desiredIP = line[1]
				} else if line[0] == "default" {
					defaultPage = line[1]
					fmt.Println("Default Page set to: " + defaultPage)
				} else if line[0] == "cors" {
					cors := corsAllowed{strings.ToLower(line[1])}
					corsList = append(corsList, cors)
				}
				s, e = Readln(r)
			}
			fmt.Println("=================")
		}

		var ipAdd = GetIPAddress(desiredIP)
		if desiredIP == "" {
			desiredIP = "first available"
		}
		fmt.Println("Desired IP Address for Web Server: " + desiredIP)
		fmt.Println("=================")
		if ipAdd == "" {
			fmt.Println("Unable to match desired IP Address to any IP addresses assigned to this machine")
			fmt.Println("Fatal error: aborting")
			os.Exit(2)
		} else {
			cors := corsAllowed{"http://" + strings.ToLower(ipAdd) + ":" + s}
			corsList = append(corsList, cors)
			for _, element := range corsList {
				fmt.Println("Allowed CORS request: " + element.origin)
			}
			fmt.Println("=================")
			fmt.Println("Web Server started on http://" + ipAdd + ":" + s)
			http.HandleFunc("/", handler)
			err := http.ListenAndServe(ipAdd+":"+s, nil)
			if err != nil {
				fmt.Println(err)
				fmt.Println("Fatal error: aborting")
				os.Exit(3)
			}
		}
	}
}
