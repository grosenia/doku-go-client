package dokugo

import "net/http"

// CreateVA creates a Virtual Account. Whether it's Static or Non-static
// (single-use) is controlled entirely by req.AdditionalInfo.VirtualAccountConfig.ReusableStatus
// — see NewStaticVARequest / NewSingleUseVARequest.
func (g *Gateway) CreateVA(req *CreateVARequest) (*CreateVAResponse, error) {
	resp := &CreateVAResponse{}
	_, err := g.doRequest(http.MethodPost, pathCreateVA, req, resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (g *Gateway) DeleteVA(req *DeleteVARequest) (*DeleteVAResponse, error) {
	resp := &DeleteVAResponse{}
	_, err := g.doRequest(http.MethodDelete, pathDeleteVA, req, resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (g *Gateway) UpdateVA(req *UpdateVARequest) (*UpdateVAResponse, error) {
	resp := &UpdateVAResponse{}
	_, err := g.doRequest(http.MethodPut, pathUpdateVA, req, resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// CheckStatus polls DOKU for a VA's payment status. DOKU's docs explicitly
// warn: hit this no sooner than 60 seconds after payment completion.
func (g *Gateway) CheckStatus(req *CheckStatusRequest) (*CheckStatusResponse, error) {
	resp := &CheckStatusResponse{}
	_, err := g.doRequest(http.MethodPost, pathCheckStatus, req, resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
