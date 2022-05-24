package serialize

import (
	"encoding/json"
	"fmt"

	"github.com/coinbase/kryptology/pkg/core/curves"
	"github.com/coinbase/kryptology/pkg/dkg/frost"
	"github.com/coinbase/kryptology/pkg/sharing"
)

var scalarBytes = 32

type ByteBcast struct {
	Verifiers [][]byte
	Wi, Ci    []byte
}

func pointToBinary(point curves.Point) []byte {
	t := point.ToAffineCompressed()
	name := []byte(point.CurveName())
	output := make([]byte, len(name)+1+len(t))
	copy(output[:len(name)], name)
	output[len(name)] = byte(':')
	copy(output[len(output)-len(t):], t)
	return output
}

func binaryToPoint(input []byte) (curves.Point, error) {
	if len(input) < scalarBytes+1+len(curves.P256Name) {
		return nil, fmt.Errorf("invalid byte sequence")
	}
	sep := byte(':')
	i := 0
	for ; i < len(input); i++ {
		if input[i] == sep {
			break
		}
	}
	name := string(input[:i])
	curve := curves.GetCurveByName(name)
	if curve == nil {
		return nil, fmt.Errorf("unrecognized curve")
	}
	return curve.Point.FromAffineCompressed(input[i+1:])
}

func feldmanVerifierToBinary(feldmanVerifier sharing.FeldmanVerifier) [][]byte {
	var bytes [][]byte
	for _, p := range feldmanVerifier.Commitments {
		bytes = append(bytes, pointToBinary(p))
	}

	return bytes
}

func binaryToFeldmanVerifier(input [][]byte) ([]curves.Point, error) {
	var Commitments []curves.Point
	for _, v := range input {
		p, err := binaryToPoint(v)
		if err != nil {
			return nil, err
		}
		Commitments = append(Commitments, p)
	}

	return Commitments, nil
}

func scalarToBinary(scalar curves.Scalar) []byte {
	name := []byte(scalar.Point().CurveName())
	output := make([]byte, len(name)+1+scalarBytes)
	copy(output[:len(name)], name)
	output[len(name)] = byte(':')
	copy(output[len(name)+1:], scalar.Bytes())
	return output
}

func unmarshalScalar(input []byte) (*curves.Curve, []byte, error) {
	sep := byte(':')
	i := 0
	for ; i < len(input); i++ {
		if input[i] == sep {
			break
		}
	}
	name := string(input[:i])
	curve := curves.GetCurveByName(name)
	if curve == nil {
		return nil, nil, fmt.Errorf("unrecognized curve")
	}
	return curve, input[i+1:], nil
}

func binaryToScalar(input []byte) (curves.Scalar, error) {
	// All scalars are 32 bytes long
	// The first 32 bytes are the actual value
	// The remaining bytes are the curve name
	if len(input) < scalarBytes+1+len(curves.P256Name) {
		return nil, fmt.Errorf("invalid byte sequence")
	}
	sc, data, err := unmarshalScalar(input)
	if err != nil {
		return nil, err
	}
	return sc.Scalar.SetBytes(data)
}

func BcastToBinary(bcast *frost.Round1Bcast) ([]byte, error) {
	var a = feldmanVerifierToBinary(*bcast.Verifiers)
	var b = scalarToBinary(bcast.Wi)
	var c = scalarToBinary(bcast.Ci)

	bbcast := ByteBcast{
		Verifiers: a,
		Wi:        b,
		Ci:        c,
	}

	return json.Marshal(bbcast)
}

func BinaryToBcast(input []byte) *frost.Round1Bcast {
	var bbcast ByteBcast
	var verifiers *sharing.FeldmanVerifier
	var bcast *frost.Round1Bcast
	json.Unmarshal(input, &bbcast)

	points, _ := binaryToFeldmanVerifier(bbcast.Verifiers)

	verifiers = &sharing.FeldmanVerifier{
		Commitments: points,
	}

	wi, _ := binaryToScalar(bbcast.Wi)
	ci, _ := binaryToScalar(bbcast.Ci)

	bcast = &frost.Round1Bcast{
		Verifiers: verifiers,
		Wi:        wi,
		Ci:        ci,
	}

	return bcast
}
