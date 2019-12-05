/*
Copyright 2019 PlanetScale.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package partitioning

import (
	"encoding/binary"
	"math"

	"vitess.io/vitess/go/vt/key"
	topodatapb "vitess.io/vitess/go/vt/proto/topodata"
)

/*
EqualKeyRanges returns the list of KeyRanges for a partitioning into a given number of equal parts.

If the number of parts is a power of 2, the keyspace will be partitioned into
exactly equal parts.

Otherwise, the parts will only be approximately equal because the total number
of keyspace IDs is a power of 2, and nothing divides evenly into a power of 2
except other powers of 2. The approximate cut points are generally fine as
long as we choose them with a deterministic algorithm that spreads the
remainder as uniformly across shards as possible, as is done here.

WARNING: DO NOT change the behavior of this function, as it may result in the
         deletion of shards.
*/
func EqualKeyRanges(parts uint64) []topodatapb.KeyRange {

	// Short-circuit for parts<=1 so we can assume parts>1 below.
	if parts <= 1 {
		// The "unsharded" case.
		return []topodatapb.KeyRange{{}}
	}

	/*
		How many bytes do we need to include in our start/end bounds
		to split into this many parts? In other words, how many bytes
		does it take to represent 0 through parts-1 (n different values)?

		This is essentially numBytes = ceil(log256(parts-1)).

		Note that parts is validated to be greater than 1.
	*/
	numBytes := 0
	for q := parts - 1; q != 0; q >>= 8 {
		numBytes++
	}

	/*
		The expression for 'interval' below simplifies to (2^64 / parts) without
		overflowing, as long as parts>1.

		Note that we use 2^64 as the numerator instead of 2^(8*numBytes),
		even though we throw away everything but the most-significant numBytes.
		The extra precision helps to spread the remainder uniformly across the
		shards when parts is not a power of 2.

		For example, if we had used 256 as the numerator, then the 10th of 10
		shards would own 24% more keyspace IDs than the rest. In a pathological
		case like n=129, using 256 as the numerator would have resulted in the
		last shard owning 50% of all keyspace IDs.
	*/
	interval := (math.MaxUint64-parts+1)/parts + 1
	keyRanges := make([]topodatapb.KeyRange, parts)
	for i := uint64(0); i < parts; i++ {
		if i > 0 {
			keyRanges[i].Start = keyRanges[i-1].End
		}
		if i < parts-1 {
			var buf [8]byte
			binary.BigEndian.PutUint64(buf[:], uint64(i+1)*interval)
			keyRanges[i].End = buf[:numBytes]
		}
	}

	return keyRanges
}

// EqualShardNames is a helper method that calls KeyRanges(), and then iterates over
// the byte array and turns each KeyRange into a hex string in "start-end" format.
func EqualShardNames(parts uint64) []string {
	keyRangesRaw := EqualKeyRanges(parts)
	keyRangesStrings := make([]string, len(keyRangesRaw))

	for i, keyRange := range keyRangesRaw {
		keyRangesStrings[i] = key.KeyRangeString(&keyRange)
	}

	return keyRangesStrings
}
