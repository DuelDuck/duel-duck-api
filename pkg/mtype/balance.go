package mtype

type Balance int64

func NewBalance(n int) (Balance, bool) {
	if n < 0 {
		return 0, false
	}

	return Balance(n), true
}

func (b *Balance) Int() int {
	return int(*b)
}

func (b *Balance) Set(n int) bool {
	if n < 0 {
		return false
	}

	*b = Balance(n)
	return true
}

func (b *Balance) Add(n int) bool {
	if n < 0 {
		return false
	}

	*b += Balance(n)
	return true
}

func (b *Balance) LessThan(n uint64) bool {
	if (*b) < Balance(n) {
		return true
	}

	return false
}
