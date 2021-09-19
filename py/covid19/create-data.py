import requests
import bs4
import re
import csv
import sys
import os
import argparse


class AkitaFetcher:
    latest_url = "https://www.pref.akita.lg.jp/pages/archive/47957"
    # latest_url = "https://www.pref.akita.lg.jp/pages/archive/60163"
    # latest_url = "https://www.pref.akita.lg.jp/pages/archive/59894"
    # latest_url = "https://www.pref.akita.lg.jp/pages/archive/59729"
    # latest_url = "https://www.pref.akita.lg.jp/pages/archive/59331"
    # latest_url = "https://www.pref.akita.lg.jp/pages/archive/58645"
    # latest_url = "https://www.pref.akita.lg.jp/pages/archive/57552"
    # latest_url = "https://www.pref.akita.lg.jp/pages/archive/57444"
    # latest_url = "https://www.pref.akita.lg.jp/pages/archive/57443"

    @staticmethod
    def latest():
        res = requests.get(AkitaFetcher.latest_url)
        soup = bs4.BeautifulSoup(res.text, "html.parser")
        rows = soup.find("table").find("tbody").find_all("tr")
        return rows

    @staticmethod
    def header():
        return ["No", "感染判明日", "年齢"]


class Fetcher:
    fetcher = {
        "akita": AkitaFetcher,
    }

    def __init__(self, prefecture):
        self.pref = prefecture
        self.impl = Fetcher.fetcher[self.pref]()

    def fetch(self):
        return self.impl.latest()

    def header(self):
        return self.impl.header()


class AkitaParser:
    x = re.compile(r'\d+')

    def parse_row(self, data):
        col = [x.get_text() for x in data.find_all("td")]
        if len(col) != 7:
            return None
        date = AkitaParser.x.findall(col[1])
        return [
            AkitaParser.x.findall(col[0])[0],
            "%s-%s" % (date[0], date[1]),
            col[2],
            col[3],
        ]


class DataParser:
    parser = {
        "akita": AkitaParser,
    }

    def __init__(self, prefecture):
        self.pref = prefecture

    def parse(self, rows):
        parser = DataParser.parser[self.pref]()
        data = []
        for row in rows:
            cols = parser.parse_row(row)
            data.append(cols)
        return data


class AkitaMutator:
    def mutate(self, data):
        for row in data:
            if len(row[1].split("-")) == 3:
                continue
            if int(row[0]) < 14:
                row[1] = "2020-"+row[1]
            else:
                row[1] = "2021-"+row[1]
        return data


class Mutator:
    mutator = {
        "akita": AkitaMutator
    }

    def __init__(self, prefecture, data):
        self.pref = prefecture
        self.data = data

    def mutate(self):
        return Mutator.mutator[self.pref]().mutate(self.data)


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument("--prefecture", dest="prefecture")
    parser.add_argument("--file", dest="file")
    args = parser.parse_args()

    exist_data = {}
    if os.path.exists(args.file):
        with open(sys.argv[1]) as f:
            r = csv.reader(f)
            for row in r:
                exist_data[row[0]] = True

    fetcher = Fetcher(args.prefecture)
    rows = fetcher.fetch()
    data = DataParser(args.prefecture).parse(rows)
    csv_data = Mutator(args.prefecture, data).mutate()

    with open(args.file, "a") as f:
        w = csv.writer(f)
        if not exist_data:
            w.writerow(fetcher.header())
        for row in csv_data:
            if row[0] in data:
                continue
            w.writerow(row)