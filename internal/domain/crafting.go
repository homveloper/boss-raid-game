package domain

import (
	"time"
)

// CraftableItem represents an item that can be crafted
type CraftableItem struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	BaseTimeMinutes int      `json:"baseTimeMinutes"` // 기본 제작 시간(분)
	Materials       []string `json:"materials"`       // 필요 재료 목록
	ImageURL        string   `json:"imageUrl"`
}

// CraftingItem represents an item being crafted
type CraftingItem struct {
	ID                  string         `json:"id"`
	ItemID              string         `json:"itemId"`              // 제작 중인 아이템 ID
	CrafterID           string         `json:"crafterId"`           // 제작 시작한 플레이어 ID
	CrafterName         string         `json:"crafterName"`         // 제작 시작한 플레이어 이름
	StartTime           time.Time      `json:"startTime"`           // 제작 시작 시각
	OriginalTimeMinutes int            `json:"originalTimeMinutes"` // 원본 총 필요 시간(분)
	CurrentTimeMinutes  int            `json:"currentTimeMinutes"`  // 현재 총 필요 시간(분)
	Helpers             map[string]int `json:"helpers"`             // 도움 준 플레이어 ID -> 도움 횟수
	Status              string         `json:"status"`              // "in_progress", "completed", "cancelled"
}

// CraftingSystem represents the crafting system
type CraftingSystem struct {
	CraftableItems map[string]CraftableItem `json:"craftableItems"` // 제작 가능한 아이템 목록
	CraftingItems  map[string]CraftingItem  `json:"craftingItems"`  // 현재 제작 중인 아이템 목록
	LastUpdated    time.Time                `json:"lastUpdated"`
}

// NewCraftingSystem creates a new crafting system
func NewCraftingSystem() *CraftingSystem {
	return &CraftingSystem{
		CraftableItems: make(map[string]CraftableItem),
		CraftingItems:  make(map[string]CraftingItem),
		LastUpdated:    time.Now(),
	}
}

// InitDefaultCraftableItems initializes the default craftable items
func (cs *CraftingSystem) InitDefaultCraftableItems() {
	// 무기 아이템
	cs.CraftableItems["sword"] = CraftableItem{
		ID:              "sword",
		Name:            "강철 검",
		Description:     "기본적인 강철 검입니다. 공격력이 증가합니다.",
		BaseTimeMinutes: 10,
		Materials:       []string{"iron", "wood"},
		ImageURL:        "/images/items/sword.png",
	}

	cs.CraftableItems["shield"] = CraftableItem{
		ID:              "shield",
		Name:            "강철 방패",
		Description:     "기본적인 강철 방패입니다. 방어력이 증가합니다.",
		BaseTimeMinutes: 8,
		Materials:       []string{"iron", "wood"},
		ImageURL:        "/images/items/shield.png",
	}

	cs.CraftableItems["bow"] = CraftableItem{
		ID:              "bow",
		Name:            "장궁",
		Description:     "원거리 공격이 가능한 활입니다.",
		BaseTimeMinutes: 12,
		Materials:       []string{"wood", "string"},
		ImageURL:        "/images/items/bow.png",
	}

	cs.CraftableItems["staff"] = CraftableItem{
		ID:              "staff",
		Name:            "마법 지팡이",
		Description:     "마법 공격력이 증가하는 지팡이입니다.",
		BaseTimeMinutes: 15,
		Materials:       []string{"wood", "crystal"},
		ImageURL:        "/images/items/staff.png",
	}

	cs.CraftableItems["dagger"] = CraftableItem{
		ID:              "dagger",
		Name:            "암살자의 단검",
		Description:     "빠른 공격이 가능한 단검입니다. 치명타 확률이 증가합니다.",
		BaseTimeMinutes: 7,
		Materials:       []string{"steel", "leather"},
		ImageURL:        "/images/items/dagger.png",
	}

	cs.CraftableItems["axe"] = CraftableItem{
		ID:              "axe",
		Name:            "전투 도끼",
		Description:     "강력한 공격력을 가진 도끼입니다. 방어력 관통 효과가 있습니다.",
		BaseTimeMinutes: 14,
		Materials:       []string{"iron", "hardwood"},
		ImageURL:        "/images/items/axe.png",
	}

	// 방어구 아이템
	cs.CraftableItems["leather_armor"] = CraftableItem{
		ID:              "leather_armor",
		Name:            "가죽 갑옷",
		Description:     "가볍고 움직임이 자유로운 가죽 갑옷입니다. 기본적인 방어력을 제공합니다.",
		BaseTimeMinutes: 9,
		Materials:       []string{"leather", "thread"},
		ImageURL:        "/images/items/leather_armor.png",
	}

	cs.CraftableItems["chain_mail"] = CraftableItem{
		ID:              "chain_mail",
		Name:            "사슬 갑옷",
		Description:     "쇠사슬로 만든 갑옷입니다. 중간 수준의 방어력을 제공합니다.",
		BaseTimeMinutes: 13,
		Materials:       []string{"iron", "leather"},
		ImageURL:        "/images/items/chain_mail.png",
	}

	cs.CraftableItems["plate_armor"] = CraftableItem{
		ID:              "plate_armor",
		Name:            "판금 갑옷",
		Description:     "무거운 금속판으로 만든 갑옷입니다. 높은 방어력을 제공합니다.",
		BaseTimeMinutes: 18,
		Materials:       []string{"steel", "leather", "cloth"},
		ImageURL:        "/images/items/plate_armor.png",
	}

	// 소비 아이템
	cs.CraftableItems["health_potion"] = CraftableItem{
		ID:              "health_potion",
		Name:            "체력 물약",
		Description:     "체력을 회복시켜주는 물약입니다.",
		BaseTimeMinutes: 5,
		Materials:       []string{"herb", "water"},
		ImageURL:        "/images/items/health_potion.png",
	}

	cs.CraftableItems["mana_potion"] = CraftableItem{
		ID:              "mana_potion",
		Name:            "마나 물약",
		Description:     "마나를 회복시켜주는 물약입니다.",
		BaseTimeMinutes: 5,
		Materials:       []string{"blue_herb", "water"},
		ImageURL:        "/images/items/mana_potion.png",
	}

	cs.CraftableItems["strength_potion"] = CraftableItem{
		ID:              "strength_potion",
		Name:            "힘 증강 물약",
		Description:     "일시적으로 공격력을 증가시켜주는 물약입니다.",
		BaseTimeMinutes: 8,
		Materials:       []string{"red_herb", "mushroom", "water"},
		ImageURL:        "/images/items/strength_potion.png",
	}

	// 특수 아이템
	cs.CraftableItems["dragon_slayer"] = CraftableItem{
		ID:              "dragon_slayer",
		Name:            "드래곤 슬레이어",
		Description:     "드래곤을 처치하기 위해 특별히 제작된 전설의 검입니다. 보스 몬스터에게 추가 피해를 입힙니다.",
		BaseTimeMinutes: 25,
		Materials:       []string{"dragon_scale", "mythril", "ancient_wood"},
		ImageURL:        "/images/items/dragon_slayer.png",
	}

	cs.CraftableItems["phoenix_robe"] = CraftableItem{
		ID:              "phoenix_robe",
		Name:            "불사조의 로브",
		Description:     "불사조의 깃털로 만든 마법 로브입니다. 사망 시 한 번 부활할 수 있는 능력이 있습니다.",
		BaseTimeMinutes: 20,
		Materials:       []string{"phoenix_feather", "silk", "magic_thread"},
		ImageURL:        "/images/items/phoenix_robe.png",
	}

	cs.CraftableItems["amulet_of_protection"] = CraftableItem{
		ID:              "amulet_of_protection",
		Name:            "수호의 아뮬렛",
		Description:     "착용자를 보호하는 마법이 깃든 아뮬렛입니다. 모든 속성 피해를 감소시킵니다.",
		BaseTimeMinutes: 15,
		Materials:       []string{"magic_crystal", "silver", "ancient_rune"},
		ImageURL:        "/images/items/amulet.png",
	}

	cs.LastUpdated = time.Now()
}

// IsCraftingCompleted checks if a crafting item is completed
func (cs *CraftingSystem) IsCraftingCompleted(craftingID string) bool {
	item, exists := cs.CraftingItems[craftingID]
	if !exists {
		return false
	}

	// 이미 완료 상태인 경우
	if item.Status == "completed" {
		return true
	}

	// 현재 시간이 (시작 시간 + 현재 필요 시간)을 지났는지 확인
	completionTime := item.StartTime.Add(time.Duration(item.CurrentTimeMinutes) * time.Minute)
	return time.Now().After(completionTime)
}

// UpdateCraftingStatus updates the status of all crafting items
func (cs *CraftingSystem) UpdateCraftingStatus() bool {
	updated := false

	for id, item := range cs.CraftingItems {
		if item.Status == "in_progress" && cs.IsCraftingCompleted(id) {
			item.Status = "completed"
			cs.CraftingItems[id] = item
			updated = true
		}
	}

	if updated {
		cs.LastUpdated = time.Now()
	}

	return updated
}

// GetLastHelpTime gets the last time a player helped a crafting item
func (cs *CraftingSystem) GetLastHelpTime(helperID, craftingID string) time.Time {
	// 실제 구현에서는 이벤트 로그나 별도의 데이터 구조를 사용하여 마지막 도움 시간을 추적할 수 있습니다.
	// 여기서는 간단히 빈 시간을 반환합니다.
	return time.Time{}
}
