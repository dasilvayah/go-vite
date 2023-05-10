package core

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vitelabs/go-vite/v2/common/types"
	"github.com/vitelabs/go-vite/v2/common/upgrade"
)

func TestVmLogList_Hash(t *testing.T) {
	var vmLogList VmLogList

	upgrade.CleanupUpgradeBox()
	upgrade.InitUpgradeBox(upgrade.NewEmptyUpgradeBox().AddPoint(1, 90))

	hash1, err1 := types.HexToHash("0dede580455f970517210ae2b9c0fbba74d5b7eea07eb0c62725e06c45061711")
	if err1 != nil {
		t.Fatal(err1)
	}

	hash2, err2 := types.HexToHash("c512660e3ee7d1dc005a6206ccaf84e6b567592d543f537566d871c2440e187e")
	if err2 != nil {
		t.Fatal(err2)
	}

	vmLogList = append(vmLogList, &VmLog{
		Topics: []types.Hash{hash1, hash2},
		Data:   []byte("test"),
	})

	address, err := types.HexToAddress("vite_544faefbc5031341c9352b0b22161bc0b4e5b342dc7fe04028")
	if err != nil {
		t.Fatal(err)
	}
	prehash, err := types.HexToHash("b7a12797d132c6545b5cc15afa8bb3811ac655f7a56cf76bad68bead4dfe5a44")
	if err != nil {
		t.Fatal(err)
	}
	vmLogHash1 := vmLogList.Hash(1, address, prehash)
	vmLogHash50 := vmLogList.Hash(50, address, prehash)
	vmLogHash100 := vmLogList.Hash(100, address, prehash)

	if *vmLogHash1 != *vmLogHash50 {
		t.Fatal(fmt.Sprintf("vmloghash1 should be equal with vmloghash50 , %+v, %+v", vmLogHash1, vmLogHash50))
	}

	if *vmLogHash100 == *vmLogHash1 {
		t.Fatal(fmt.Sprintf("vmloghash1 should not be equal with vmloghash100 , %+v, %+v", vmLogHash100, vmLogHash1))
	}

	upgrade.CleanupUpgradeBox()
	upgrade.InitUpgradeBox(upgrade.NewEmptyUpgradeBox().AddPoint(1, 101))

	vmLogHash95 := vmLogList.Hash(95, address, prehash)

	if *vmLogHash50 != *vmLogHash95 {
		t.Fatal(fmt.Sprintf("vmloghash51 should be equal with vmloghash50 , %+v, %+v", vmLogHash95, vmLogHash50))
	}

	vmLogHash105 := vmLogList.Hash(105, address, prehash)
	if *vmLogHash105 != *vmLogHash100 {
		t.Fatal(fmt.Sprintf("vmloghash105 should be equal with vmLogHash100 , %+v, %+v", vmLogHash105, vmLogHash100))
	}

}

func TestVmLogList_Serialize(t *testing.T) {
	hash1 := types.HexToHashPanic("0dede580455f970517210ae2b9c0fbba74d5b7eea07eb0c62725e06c45061711")
	hash2 := types.HexToHashPanic("c512660e3ee7d1dc005a6206ccaf84e6b567592d543f537566d871c2440e187e")

	var vmLogList VmLogList
	vmLogList = append(vmLogList, &VmLog{
		Topics: []types.Hash{hash1, hash2},
		Data:   []byte("test"),
	})

	byt, err := vmLogList.Serialize()
	assert.NoError(t, err)

	var vmLogList2 = &VmLogList{}

	err = vmLogList2.Deserialize(byt)
	assert.NoError(t, err)

	assert.Equal(t, len(vmLogList), len(*vmLogList2))
	for i, vmLog := range vmLogList {
		assert.Equal(t, (*vmLogList2)[i].Data, vmLog.Data)
		assert.Equal(t, (*vmLogList2)[i].Topics, vmLog.Topics)
	}
}
