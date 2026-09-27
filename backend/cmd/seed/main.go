// Команда seed заводит демо-собрание из testdata/demo_house.csv,
// чтобы проверять бота и дашборд без мини-приложения.
//
// Голоса отдают демо-пользователи с отрицательными max_id: настоящих
// id MAX такими не бывает, и их легко найти и удалить.
package main

import (
	"context"
	"flag"
	"log"
	"math/rand/v2"
	"os"
	"time"

	"oss-max/internal/config"
	"oss-max/internal/domain"
	"oss-max/internal/registry"
	"oss-max/internal/storage"
)

func main() {
	var (
		initiator = flag.Int64("initiator", 0, "max_id инициатора — ваш id в MAX (обязательно)")
		csvPath   = flag.String("csv", "testdata/demo_house.csv", "реестр")
		question  = flag.String("question", "Установить шлагбаум на въезде во двор", "вопрос собрания")
		rule      = flag.String("rule", "soft", "правило порога: soft, hard, all")
		publish   = flag.Bool("publish", true, "сразу открыть голосование")
		turnout   = flag.Float64("turnout", 0.4, "доля собственников, которые проголосуют (0..1)")
		pending   = flag.Float64("pending", 0.1, "доля голосов, оставшихся без подтверждения (0..1)")
	)
	flag.Parse()

	if *initiator == 0 {
		log.Fatal("укажите -initiator: свой max_id можно узнать в логе бота после /start")
	}

	url, err := config.DatabaseURL()
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	db, err := storage.Open(ctx, url)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	file, err := os.Open(*csvPath)
	if err != nil {
		log.Fatal(err)
	}
	rows, err := registry.ParseCSV(file)
	file.Close()
	if err != nil {
		log.Fatalf("реестр: %v", err)
	}

	flats, report, err := registry.Group(rows)
	if err != nil {
		log.Fatalf("реестр: %v", err)
	}
	for _, warning := range report.Warnings {
		log.Printf("предупреждение: %s", warning)
	}

	endsAt := time.Now().Add(14 * 24 * time.Hour)
	meeting, err := db.CreateMeeting(ctx, storage.NewMeeting{
		InitiatorMaxID: *initiator,
		Address:        report.HouseAddress,
		Question:       *question,
		Rule:           pickRule(*rule),
		// Площадь дома не задаём — посчитается из реестра при импорте.
		EndsAt: &endsAt,
	})
	if err != nil {
		log.Fatal(err)
	}

	if err := db.ImportRegistry(ctx, meeting.ID, flats); err != nil {
		log.Fatalf("импорт реестра: %v", err)
	}
	log.Printf("собрание %d: %d помещений, %d собственников, %s м²",
		meeting.ID, report.Flats, report.Owners, report.FlatsArea)

	if !*publish {
		return
	}
	if err := db.Publish(ctx, meeting.ID); err != nil {
		log.Fatal(err)
	}

	units, owners, err := db.Registry(ctx, meeting.ID)
	if err != nil {
		log.Fatal(err)
	}
	numbers := make(map[int]string, len(units))
	for _, unit := range units {
		numbers[unit.UnitID] = unit.Number
	}

	// Фиксированное зерно: демо на защите должно выглядеть одинаково.
	random := rand.New(rand.NewPCG(uint64(meeting.ID), 2026))
	var voted, unconfirmed int

	// Путь тот же, что у живого собственника: заявка на квартиру,
	// голос до подтверждения, затем подтверждение инициатором.
	for _, owner := range owners {
		if random.Float64() >= *turnout {
			continue
		}

		demoUser := storage.User{MaxID: -int64(owner.OwnerID), Name: "Демо-собственник"}
		claim, _, err := db.ClaimFlat(ctx, meeting.ID, numbers[owner.UnitID], demoUser)
		if err != nil {
			log.Fatalf("заявка на квартиру %s: %v", numbers[owner.UnitID], err)
		}
		if err := db.Vote(ctx, meeting.ID, demoUser.MaxID, pickChoice(random)); err != nil {
			log.Fatalf("голос собственника %d: %v", owner.OwnerID, err)
		}
		voted++

		if random.Float64() < *pending {
			unconfirmed++
			continue
		}
		if err := db.ConfirmClaim(ctx, meeting.ID, claim.ID, owner.OwnerID); err != nil {
			log.Fatalf("подтверждение заявки %d: %v", claim.ID, err)
		}
	}

	result, err := db.Result(ctx, meeting.ID)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("голосов %d, из них ждут подтверждения %d; учтено %.2f из %.2f м², кворум: %v",
		voted, unconfirmed, result.Tally.Total, result.TotalArea, result.Quorum)
}

func pickRule(name string) domain.Rule {
	switch name {
	case "hard":
		return domain.RuleHard()
	case "all":
		return domain.RuleAll()
	default:
		return domain.RuleSoft()
	}
}

func pickChoice(random *rand.Rand) domain.Choice {
	switch x := random.Float64(); {
	case x < 0.7:
		return domain.ChoiceFor
	case x < 0.85:
		return domain.ChoiceAgainst
	default:
		return domain.ChoiceAbstain
	}
}
