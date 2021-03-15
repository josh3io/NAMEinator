package main

import (
	"sync"
	"time"
  "sort"
)

type NSinfo struct {
	IPAddr           string
	Name             string
	Country          string
	Count            int
	ErrorsConnection int
	ErrorsValidation int
	ID               int64
	rtt              []time.Duration
  rtt95            time.Duration
  rttMed           time.Duration
	rttAvg           time.Duration
	rttMin           time.Duration
	rttMax           time.Duration
}

type nsInfoMap struct {
	ns    map[string]NSinfo
	mutex sync.RWMutex
}

// Get IP address entry // DEBUG
func nsStoreGetRecord(nsStore nsInfoMap, ipAddr string) NSinfo {
	nsStore.mutex.RLock()
	defer nsStore.mutex.RUnlock()
	entry, found := nsStore.ns[ipAddr]
	if !found {
		entry.IPAddr = ipAddr
	}
	return entry
}

// Get nameserver average time
func nsStoreGetMeasurement(nsStore nsInfoMap, ipAddr string) NSinfo {
	var nsMeasurement = NSinfo{}
	entry, found := nsStore.ns[ipAddr]
	if !found {
		entry.IPAddr = ipAddr
	}
	var total time.Duration = 0
	var min time.Duration = 10000000
	var max time.Duration = 0
  var ms []int64
	for _, value := range entry.rtt {
		// check for new min record
		if value < min {
			min = value
		}
		// check for new max record
		if value > max {
			max = value
		}
		// add for total time
		total += value
    ms = append(ms,value.Microseconds()*1000)
	}

  sort.Slice(ms, func(i,j int) bool { return ms[i]<ms[j] })
  var i95 int = int(float64(len(ms))*.95)
  var i50 int = int(float64(len(ms))/2)

  nsMeasurement.rtt95  = time.Duration(ms[i95])
  nsMeasurement.rttMed = time.Duration(ms[i50])
	nsMeasurement.rttAvg = total / time.Duration(len(entry.rtt))
	nsMeasurement.rttMin = min
	nsMeasurement.rttMax = max
	return nsMeasurement
}

// add rtt to the nameserver slice
func nsStoreSetRTT(nsStore nsInfoMap, ipAddr string, rtt time.Duration) {
	nsStore.mutex.Lock()
	defer nsStore.mutex.Unlock()
	entry, found := nsStore.ns[ipAddr]
	if !found {
		entry.IPAddr = ipAddr
	}
	entry.rtt = append(entry.rtt, rtt)
	entry.Count++
	nsStore.ns[ipAddr] = entry
}

// add rtt to the nameserver slice
func nsStoreAddNS(nsStore nsInfoMap, ipAddr string, name string, country string) {
	nsStore.mutex.Lock()
	defer nsStore.mutex.Unlock()
	entry, found := nsStore.ns[ipAddr]
	if !found {
		entry.IPAddr = ipAddr
	}
	entry.Name = name
	entry.Country = country
	nsStore.ns[ipAddr] = entry
}
