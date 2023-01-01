package opvault

type Category string

const (
	CategoryLogin           Category = "001"
	CategoryCreditCard      Category = "002"
	CategorySecureNote      Category = "003"
	CategoryIdentity        Category = "004"
	CategoryPassword        Category = "005"
	CategoryTombstone       Category = "099"
	CategorySoftwareLicense Category = "100"
	CategoryBankAccount     Category = "101"
	CategoryDatabase        Category = "102"
	CategoryDriverLicense   Category = "103"
	CategoryOutdoorLicense  Category = "104"
	CategoryMembership      Category = "105"
	CategoryPassport        Category = "106"
	CategoryRewards         Category = "107"
	CategorySSN             Category = "108"
	CategoryRouter          Category = "109"
	CategoryServer          Category = "110"
	CategoryEmail           Category = "111"
)
