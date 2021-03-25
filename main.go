package main

import (
  "time"
	"flag"
	"fmt"
	"github.com/miekg/dns"
	"log"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
)

var VERSION = "custom"
var appConfiguration APPconfig

type SortType string
const (
  min SortType ="min"
  max SortType ="max"
  avg SortType ="avg"
  med SortType ="median"
)

type APPconfig struct {
	numberOfDomains int
	debug           bool
	contest         bool
	nameserver      string
  showPercentiles bool
  sortType        SortType
}

// process flags
func processFlags() {
	var appConfigstruct APPconfig
	flagNumberOfDomains := flag.Int("domains", 100, "number of domains to be tested")
	flagNameserver := flag.String("nameserver", "", "specify a nameserver instead of using defaults")
	flagContest := flag.Bool("contest", true, "contest=true/false : enable or disable a contest against your locally configured DNS server (default true)")
	flagDebug := flag.Bool("debug", false, "debug=true/false : enable or disable debugging (default false)")
  flagFullPercentilesTable := flag.Bool("percentiles", false, "percentiles=true/false : show percentiles table")
  flagSort := flag.String("sort", "avg", "sort=(min|max|avg|median)")
	flag.Parse()
	appConfigstruct.numberOfDomains = *flagNumberOfDomains
	appConfigstruct.debug = *flagDebug
	appConfigstruct.contest = *flagContest
	appConfigstruct.nameserver = *flagNameserver
  appConfigstruct.showPercentiles = *flagFullPercentilesTable
  appConfigstruct.sortType = SortType(*flagSort)
	appConfiguration = appConfigstruct
}

// return the IP of the DNS used by the operating system
func getOSdns() string {
	// get local dns ip
	out, err := exec.Command("nslookup", ".").Output()
	if appConfiguration.debug {
		fmt.Println("DEBUG: nslookup output")
		fmt.Printf("%s\n", out)
	}
	var errorcode = fmt.Sprint(err)
	if err != nil {
		if errorcode == "exit status 1" {
			// newer versions of nslookup return error code 1 when executing "nslookup ." - but that's fine for us
			_ = err
		} else {
			log.Print("Something went wrong obtaining the local DNS Server - is \"nslookup\" available?")
			log.Fatal(err)
		}
	}

	// fmt.Printf("%s\n", out)
	re := regexp.MustCompile("\\b\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}\\b") // TODO: Make IPv6 compatible
	// fmt.Printf("%q\n", re.FindString(string(out)))
	var localdns = re.FindString(string(out))
	if appConfiguration.debug {
		fmt.Println("DEBUG: dns server")
		fmt.Printf("%s\n", localdns)
	}
	return localdns
}

// prints welcome messages
func printWelcome() {
	fmt.Println("starting NAMEinator - version " + VERSION)
	fmt.Printf("understood the following configuration: %+v\n", appConfiguration)
	fmt.Println("-------------")
	fmt.Println("NOTE: as this is an alpha - we rely on feedback - please report bugs and featurerequests to https://github.com/mwiora/NAMEinator/issues and provide this output")
	fmt.Println("OS: " + runtime.GOOS + " ARCH: " + runtime.GOARCH)
	fmt.Println("-------------")
}

func sortKey(info NSinfo) time.Duration {
  switch appConfiguration.sortType {
    case min: return info.rttMin
    case max: return info.rttMax
    case avg: return info.rttAvg
    case med: return info.percentiles[50]
    default: return info.rttAvg
  }
}

func processResults(nsStore nsInfoMap) []NSinfo {
	nsStore.mutex.Lock()
	defer nsStore.mutex.Unlock()
	var nsStoreSorted []NSinfo
	for _, entry := range nsStore.ns {
		nsResults := nsStoreGetMeasurement(nsStore, entry.IPAddr)
    entry.percentiles = nsResults.percentiles
		entry.rttAvg = nsResults.rttAvg
		entry.rttMin = nsResults.rttMin
		entry.rttMax = nsResults.rttMax
		entry.ID = int64(sortKey(nsResults))
		nsStore.ns[entry.IPAddr] = entry
		nsStoreSorted = append(nsStoreSorted, NSinfo{entry.IPAddr, entry.Name, entry.Country, entry.Count, entry.ErrorsConnection, entry.ErrorsValidation, entry.ID, entry.rtt, entry.rttAvg, entry.rttMin, entry.rttMax, entry.percentiles})
	}
	sort.Slice(nsStoreSorted, func(i, j int) bool {
		return nsStoreSorted[i].ID < nsStoreSorted[j].ID
	})
	return nsStoreSorted
}

// prints results
func printResults(nsStore nsInfoMap, nsStoreSorted []NSinfo) {
	fmt.Println("")
	fmt.Println("finished - presenting results: ") // TODO: Colorful representation in a table PLEASE

	for _, nameserver := range nsStoreSorted {
		fmt.Println("")
		fmt.Println(nameserver.IPAddr + ": ")

		fmt.Printf("Min. [%v], Avg. [%v], Med. [%v], 95%% [%v], Max. [%v]\n", nameserver.rttMin, nameserver.rttAvg, nameserver.percentiles[50], nameserver.percentiles[95], nameserver.rttMax)

    if appConfiguration.showPercentiles {
      for _,v := range([]int{10,25,50,75,90,95,99}) {
        fmt.Printf("%d%%:  %s\n", v, nameserver.percentiles[v].String())
      }
    }
		if appConfiguration.debug {
			fmt.Println(nsStoreGetRecord(nsStore, nameserver.IPAddr))
		}
		fmt.Println("")
	}
}

// prints bye messages
func printBye() {
	fmt.Println("")
	fmt.Println("Au revoir!")
}

func prepareBenchmark(nsStore nsInfoMap, dStore dInfoMap) {
	if appConfiguration.contest {
		// we need to know who we are testing
		var localdns = getOSdns()
		loadNameserver(nsStore, localdns, "localhost")
	}
	prepareBenchmarkNameservers(nsStore)
	prepareBenchmarkDomains(dStore)
}

func performBenchmark(nsStore nsInfoMap, dStore dInfoMap) {
	// initialize DNS client
	c := new(dns.Client)
	// to avoid overload against one server we will test all defined nameservers with one domain before proceeding
	for _, domain := range dStore.d {

		m1 := new(dns.Msg)
		m1.Id = dns.Id()
		m1.RecursionDesired = true
		m1.Question = make([]dns.Question, 1)
		m1.Question[0] = dns.Question{domain.FQDN, dns.TypeA, dns.ClassINET}

		// iterate through all given nameservers
		for _, nameserver := range nsStore.ns {
			in, rtt, err := c.Exchange(m1, "["+nameserver.IPAddr+"]"+":53")
			_ = in
			nsStoreSetRTT(nsStore, nameserver.IPAddr, rtt)
			_ = err // TODO: Take care about errors during queries against the DNS - we will accept X fails
		}
		fmt.Print(".")
	}
}

func main() {
	// process startup parameters and welcome
	processFlags()
	printWelcome()

	// prepare storage for nameservers and domains
	var nsStore = nsInfoMap{ns: make(map[string]NSinfo)}
	var dStore = dInfoMap{d: make(map[string]Dinfo)}
	// var nsStoreSorted []NSinfo

	// based on startupconfiguration we have to do some preparation
	prepareBenchmark(nsStore, dStore)
	// lets go benchmark - iterate through all domains
	fmt.Println("LETS GO - each dot is a completed domain request against all nameservers")
	performBenchmark(nsStore, dStore)

	// benchmark has been completed - now we have to tell the results and say good bye
	var nsStoreSorted = processResults(nsStore)
	printResults(nsStore, nsStoreSorted)
	printBye()
}
