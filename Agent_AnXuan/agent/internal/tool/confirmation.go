package tool

import "context"

// ConfirmationRequest 描述需要狰和明确授权的副作用操作。
type ConfirmationRequest struct {
	Action  string
	Summary string
	Details string
}

// Confirmer 从模型之外获取狰和授权。
//
// 实现不得允许模型自行构造确认结果，授权必须来自狰和或其他可信主体。
type Confirmer interface {
	Confirm(ctx context.Context, request ConfirmationRequest) (bool, error)
}
