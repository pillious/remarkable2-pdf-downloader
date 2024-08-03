package main

type Set[T comparable] map[T]struct{}

func (set *Set[T]) add(item T) {
	(*set)[item] = struct{}{}
}

func (set *Set[T]) remove(item T) {
	delete(*set, item)
}

func (set *Set[T]) has(item T) bool {
	_, ok := (*set)[item]
	return ok
}

func (set *Set[T]) isEmpty() bool {
	return len(*set) == 0
}

func (set *Set[T]) union(other *Set[T]) Set[T] {
	u := Set[T]{}
	for item := range *set {
		u.add(item)
	}
	for item := range *other {
		u.add(item)
	}
	return u
}

func (set *Set[T]) intersect(other *Set[T]) Set[T] {
	inter := Set[T]{}
	a, b := set, other
	if len(*a) > len(*b) {
		a, b = other, set
	}
	for item := range *a {
		if b.has(item) {
			inter.add(item)
		}
	}
	return inter
}

func (set *Set[T]) difference(other *Set[T]) Set[T] {
	diff := Set[T]{}
	for item := range *set {
		if !other.has(item) {
			diff.add(item)
		}
	}
	return diff
}
