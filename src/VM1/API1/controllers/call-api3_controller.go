package controllers

import(
	"encoding/json"
	"net/http"
	"api1/models"
)

func CallApi3(w http.ResponseWriter, r *http.Request){
	w.Header().Set("Content-Type", "application/json")
	resp, err := http.Get("http://192.168.122.11:8081/health")
	if err != nil{
		w.WriteHeader(http.StatusServiceUnavailable)
		response := models.CallResponse{
			Api: "API3",
			Message: "ERROR: The API3 located on the VM2 is not working",
			Conecttion: false,
			Carnet: 202201117,
		}
		json.NewEncoder(w).Encode(response)
		return
	}
	defer resp.Body.Close()
	w.WriteHeader(http.StatusOK)
	response := models.CallResponse{
		Api: "API3",
		Message: "The API3 located on the VM2 is working",
		Conecttion: true,
		Carnet: 202201117,
	}
	json.NewEncoder(w).Encode(response)
	
}