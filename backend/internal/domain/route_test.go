package domain

import "testing"

func TestUncovered(t *testing.T) {
	units := []Unit{
		{UnitID: 1, Number: "1", Area: 60},
		{UnitID: 2, Number: "2", Area: 40},
		{UnitID: 3, Number: "3", Area: 80},
	}
	owners := []Owner{
		{OwnerID: 1, UnitID: 1, Share: 0.5}, // проголосовал
		{OwnerID: 2, UnitID: 1, Share: 0.5}, // нет
		{OwnerID: 3, UnitID: 2, Share: 1},   // проголосовал
		{OwnerID: 4, UnitID: 3, Share: 1},   // голос не подтверждён
	}
	votes := []Vote{
		vote(1, ChoiceFor, 30, StatusConfirmed),
		vote(3, ChoiceFor, 40, StatusConfirmed),
		vote(4, ChoiceFor, 80, StatusPending),
	}

	gaps := Uncovered(units, owners, votes)

	if len(gaps) != 2 {
		t.Fatalf("непокрытых помещений %d, хотели 2", len(gaps))
	}
	// Квартира 1: остался один собственник с долей 1/2 от 60 м².
	if gaps[0].Unit.UnitID != 1 || gaps[0].Area != 30 {
		t.Errorf("первое помещение: %+v, хотели квартиру 1 на 30 м²", gaps[0])
	}
	// Квартира 3: голос в pending, значит помещение не покрыто.
	if gaps[1].Unit.UnitID != 3 || gaps[1].Area != 80 {
		t.Errorf("второе помещение: %+v, хотели квартиру 3 на 80 м²", gaps[1])
	}
}

func TestRoute(t *testing.T) {
	gaps := []UnitGap{
		{Unit: Unit{UnitID: 1, Number: "1"}, Area: 50},
		{Unit: Unit{UnitID: 2, Number: "2"}, Area: 90},
		{Unit: Unit{UnitID: 3, Number: "3"}, Area: 40},
	}

	t.Run("разрыв закрывается двумя помещениями", func(t *testing.T) {
		route, left := Route(gaps, 100)

		if len(route) != 2 {
			t.Fatalf("в маршруте %d помещений, хотели 2", len(route))
		}
		if route[0].Area != 90 || route[1].Area != 50 {
			t.Errorf("маршрут %+v, хотели сначала крупные", route)
		}
		if left != 0 {
			t.Errorf("остаток %v, хотели 0", left)
		}
	})

	t.Run("всех помещений не хватает", func(t *testing.T) {
		route, left := Route(gaps, 300)

		if len(route) != 3 {
			t.Errorf("в маршруте %d помещений, хотели все 3", len(route))
		}
		if left != 120 {
			t.Errorf("остаток %v, хотели 120", left)
		}
	})

	t.Run("разрыв уже закрыт", func(t *testing.T) {
		route, left := Route(gaps, 0)

		if len(route) != 0 || left != 0 {
			t.Errorf("маршрут %+v, остаток %v — хотели пусто", route, left)
		}
	})
}
