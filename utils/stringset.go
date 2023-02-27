package utils

import "sort"

// https://github.com/ungerik/go-dry/blob/master/string.go
// StringSet wraps map[string]struct{} with some
// useful methods.
// TODO use this for wallets / users etc
type StringSet map[string]struct{}

func (set StringSet) Has(s string) bool {
	_, found := set[s]
	return found
}

func (set StringSet) Set(s string) {
	set[s] = struct{}{}
}

func (set StringSet) Delete(s string) {
	delete(set, s)
}

func (set StringSet) Join(other StringSet) {
	for s := range other {
		set[s] = struct{}{}
	}
}

func (set StringSet) Exclude(other StringSet) {
	for s := range other {
		delete(set, s)
	}
}

func (set StringSet) Clone() StringSet {
	clone := make(StringSet, len(set))
	for s := range set {
		clone[s] = struct{}{}
	}
	return clone
}

func (set StringSet) Sorted() []string {
	list := make([]string, len(set))
	i := 0
	for s := range set {
		list[i] = s
		i++
	}
	sort.Strings(list)
	return list
}

func (set StringSet) ReverseSorted() []string {
	list := make([]string, len(set))
	i := 0
	for s := range set {
		list[i] = s
		i++
	}
	sort.Sort(sort.Reverse(sort.StringSlice(list)))
	return list
}