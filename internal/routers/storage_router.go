package routers

import (
	"net/http"

	"github.com/gorilla/mux"

	"hash_interface/internal/handlers"
)

func SetUpInterfaceRouter(router *mux.Router) {

	router.HandleFunc("/clients", handlers.AddNewStorage).Methods(http.MethodPost)

	router.HandleFunc("/clients", handlers.GetStorageInfo).Methods(http.MethodGet)

	/* @POST
	 * Set Value
	 * Request URI : http://~/hash/data
	 * Request Data format : {
			data : [
				{ key : , value : },
				{ key : , value : }, ... ,
			]
		}
	*/
	router.HandleFunc("/hash/data", handlers.HandleUpdateKeyValue).Methods(http.MethodPost)

	/* @GET
	 * Get Value From Key
	 * Request URI : http://~/hash/data/key
	 */
	router.HandleFunc("/hash/data/{key}", handlers.GetValueFromKey).Methods(http.MethodGet)

	/* @DELETE
	 * DELETE Value From Key
	 * Request URI : http://~/hash/data/key
	 */
}
