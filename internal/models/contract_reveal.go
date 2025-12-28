package models

// RevealContractInfoRequest - запрос на раскрытие информации о заказчике
type RevealContractInfoRequest struct {
	InfoCategory string `json:"info_category" binding:"required"` // "faction", "goal", или "item"
}

// RevealContractInfoResponse - ответ с раскрытой информацией
type RevealContractInfoResponse struct {
	Message      string                    `json:"message"`
	RevealedInfo *ContractRevealedInfoData `json:"revealed_info"`
}

// RevealedInfoData - структура раскрытой информации
type ContractRevealedInfoData struct {
	InfoType string                 `json:"info_type"` // "faction", "goal", или "item"
	Data     map[string]interface{} `json:"data"`
}
