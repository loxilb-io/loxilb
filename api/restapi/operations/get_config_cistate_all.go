// Code generated by go-swagger; DO NOT EDIT.

package operations

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the generate command

import (
	"context"
	"net/http"
	"strconv"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime/middleware"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"

	"github.com/loxilb-io/loxilb/api/models"
)

// GetConfigCistateAllHandlerFunc turns a function with the right signature into a get config cistate all handler
type GetConfigCistateAllHandlerFunc func(GetConfigCistateAllParams, interface{}) middleware.Responder

// Handle executing the request and returning a response
func (fn GetConfigCistateAllHandlerFunc) Handle(params GetConfigCistateAllParams, principal interface{}) middleware.Responder {
	return fn(params, principal)
}

// GetConfigCistateAllHandler interface for that can handle valid get config cistate all params
type GetConfigCistateAllHandler interface {
	Handle(GetConfigCistateAllParams, interface{}) middleware.Responder
}

// NewGetConfigCistateAll creates a new http.Handler for the get config cistate all operation
func NewGetConfigCistateAll(ctx *middleware.Context, handler GetConfigCistateAllHandler) *GetConfigCistateAll {
	return &GetConfigCistateAll{Context: ctx, Handler: handler}
}

/*
	GetConfigCistateAll swagger:route GET /config/cistate/all getConfigCistateAll

# Get Cluster Instance State in the device

Get Cluster Instance State in the device
*/
type GetConfigCistateAll struct {
	Context *middleware.Context
	Handler GetConfigCistateAllHandler
}

func (o *GetConfigCistateAll) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	route, rCtx, _ := o.Context.RouteInfo(r)
	if rCtx != nil {
		*r = *rCtx
	}
	var Params = NewGetConfigCistateAllParams()
	uprinc, aCtx, err := o.Context.Authorize(r, route)
	if err != nil {
		o.Context.Respond(rw, r, route.Produces, route, err)
		return
	}
	if aCtx != nil {
		*r = *aCtx
	}
	var principal interface{}
	if uprinc != nil {
		principal = uprinc.(interface{}) // this is really a interface{}, I promise
	}

	if err := o.Context.BindValidRequest(r, route, &Params); err != nil { // bind params
		o.Context.Respond(rw, r, route.Produces, route, err)
		return
	}

	res := o.Handler.Handle(Params, principal) // actually handle the request
	o.Context.Respond(rw, r, route.Produces, route, res)

}

// GetConfigCistateAllOKBody get config cistate all o k body
//
// swagger:model GetConfigCistateAllOKBody
type GetConfigCistateAllOKBody struct {

	// attr
	Attr []*models.CIStatusGetEntry `json:"Attr"`
}

// Validate validates this get config cistate all o k body
func (o *GetConfigCistateAllOKBody) Validate(formats strfmt.Registry) error {
	var res []error

	if err := o.validateAttr(formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (o *GetConfigCistateAllOKBody) validateAttr(formats strfmt.Registry) error {
	if swag.IsZero(o.Attr) { // not required
		return nil
	}

	for i := 0; i < len(o.Attr); i++ {
		if swag.IsZero(o.Attr[i]) { // not required
			continue
		}

		if o.Attr[i] != nil {
			if err := o.Attr[i].Validate(formats); err != nil {
				if ve, ok := err.(*errors.Validation); ok {
					return ve.ValidateName("getConfigCistateAllOK" + "." + "Attr" + "." + strconv.Itoa(i))
				} else if ce, ok := err.(*errors.CompositeError); ok {
					return ce.ValidateName("getConfigCistateAllOK" + "." + "Attr" + "." + strconv.Itoa(i))
				}
				return err
			}
		}

	}

	return nil
}

// ContextValidate validate this get config cistate all o k body based on the context it is used
func (o *GetConfigCistateAllOKBody) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	var res []error

	if err := o.contextValidateAttr(ctx, formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (o *GetConfigCistateAllOKBody) contextValidateAttr(ctx context.Context, formats strfmt.Registry) error {

	for i := 0; i < len(o.Attr); i++ {

		if o.Attr[i] != nil {
			if err := o.Attr[i].ContextValidate(ctx, formats); err != nil {
				if ve, ok := err.(*errors.Validation); ok {
					return ve.ValidateName("getConfigCistateAllOK" + "." + "Attr" + "." + strconv.Itoa(i))
				} else if ce, ok := err.(*errors.CompositeError); ok {
					return ce.ValidateName("getConfigCistateAllOK" + "." + "Attr" + "." + strconv.Itoa(i))
				}
				return err
			}
		}

	}

	return nil
}

// MarshalBinary interface implementation
func (o *GetConfigCistateAllOKBody) MarshalBinary() ([]byte, error) {
	if o == nil {
		return nil, nil
	}
	return swag.WriteJSON(o)
}

// UnmarshalBinary interface implementation
func (o *GetConfigCistateAllOKBody) UnmarshalBinary(b []byte) error {
	var res GetConfigCistateAllOKBody
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*o = res
	return nil
}
