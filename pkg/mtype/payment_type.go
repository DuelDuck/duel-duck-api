package mtype

const (
	PaymentTypeDuck PaymentType = 0
	PaymentTypeUSDC PaymentType = 1
)

type PaymentType uint8

func NewPaymentType(n int) (PaymentType, bool) {
	pt := PaymentType(n)
	if !pt.IsValid() {
		return 0, false
	}

	return pt, true
}

func (p *PaymentType) Set(n int) bool {
	if !PaymentType(n).IsValid() {
		return false
	}

	*p = PaymentType(n)
	return true
}

func (p PaymentType) IsValid() bool {
	switch p {
	case PaymentTypeDuck, PaymentTypeUSDC:
		return true
	}

	return false
}
