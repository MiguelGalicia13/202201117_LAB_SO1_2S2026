package models
type CallResponse struct{
	Api string `json:"apiname"`
	Message string `json:"message"`
	Conecttion bool `json:"connection"`
	Carnet int `json:"carnet"`
}