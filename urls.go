package dokugo

// SNAP endpoint paths, relative to Client.BaseURL.
const (
	pathCreateVA    = "/virtual-accounts/bi-snap-va/v1.1/transfer-va/create-va"
	pathDeleteVA    = "/virtual-accounts/bi-snap-va/v1.1/transfer-va/delete-va"
	pathUpdateVA    = "/virtual-accounts/bi-snap-va/v1.1/transfer-va/update-va"
	pathCheckStatus = "/orders/v1.0/transfer-va/status" // different path family than the VA endpoints above
)
