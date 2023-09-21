// Ursa rate limiter is a http.Handler
package ursa

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/ursaserver/ursa/memoize"
)

type reqSignature string
type reqPath string

type server struct {
	boxes   map[reqSignature]box
	rateBys []RateBy
	sync.RWMutex
	conf     Conf
	pathRate func(reqPath) rate
}

type bucketId string

type box struct {
	id      reqSignature // request signature
	buckets map[reqPath]bucket
	sync.RWMutex
}

type bucket struct {
	id           reqPath // request path
	tokens       int
	rate         rate
	lastAccessed time.Time
	box          *box
	sync.Mutex
}

// Create a server based on provided configuration.
// Initializes gifters
func New(conf Conf) server {
	// TODO initialize gifters
	s := &server{conf: conf}
	s.boxes := make(map[reqSignature]box)
	s.pathRate = memoize.Unary(func(r reqPath) rate {
		// Note that memoization is possible since the configuration is not changed once loaded.
		return rateForPath(r, conf)
	})
	// TODO init reverse proxy
}

// Return the rate based on configuration that should be used for the a given reqPath.
func rateForPath(r reqPath, conf Conf) rate {
	// Search linearly through the routes in the configuration to find a
	// pattern that matches reqPath. Note that speed won't be an issue here
	// since this function is supposed to be memoized when using.
	// Memoization should be possible since the configuration is not changed once loaded.
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sig := findReqSignature(r)
	// Find a box for given signature
	s.RLock()
	if _, ok := s.boxes[sig]; !ok {
		s.RUnlock()
		// create box with given signature
		s.Lock()
		s[sig] = box{id: sig}
		s.Unlock()
		s.RLock()
	}
	b := s.boxes[sig]
	path := findPath(r)
	b.RLock()
	if _, ok := b.buckets[path]; !ok {
		s.RUnlock()
		// create bucket,
		s.createBucket(path, b)
		s.RLock()
	}
	// While we're here, we can safely assume that the gifter isn't deleting
	// thus this bucket as it would require gifter to acquire a Write Lock
	// which can't be granted while there's still a reader.
	buck := b.buckets[path]
	b.RUnlock()

	buck.Lock()
	defer buck.Unlock()
	// We check if the no. of tokens is >= 1
	// Just before leaving, we set the last accessed time on the bucket
	buck.tokens--
	if buck.tokens < 0 {
		// TODO
		// Reject downstream & return
	}
	// TODO
	// Call HTTPServer of the underlying ReverseProxy
	buck.lastAccessed = time.Now()
	buck.Unlock()
}

// Create a bucket with given id inside the given box.
// Initializes various properties of the bucket like capacity, state time, etc.
// and then registers the bucket to the gifter to collect gift tokens.
func (s *server) createBucket(id reqPath, b box) {
	b.Lock()
	rate := s.pathRate(id)
	acc := time.Now()
	tokens := rate.capacity
	b := bucket{id, tokens, rate, acc, b}
	b.Unlock()
}

func findReqSignature(r *http.Request, rateBys []RateBy) reqSignature {
	// Find if any of the header fields in RateBy are present.
	rateby := rateByIP // default
	key := ""
	for _, r := range rateBys {
		if v := r.Header.Get(r); r != "" {
			rateBy = r
			key = r
			break
		}
	}
	// TODO, set appropriate key based on the downstream IP address.
	if rateBy == rateByIP {
		key = "ipaddressvaluetodo"
	}
	return reqSignature(fmt.Sprintf("%v-%v", rateBy, key))
}

func findPath(r *http.Request) reqPath {
	// TODO
	// decide what how to handle trailing, leading forward slashes
	return reqPath(r.URL.Path)
}