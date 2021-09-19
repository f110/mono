import sys
import pandas
import matplotlib.pyplot as plt

start_date = "2021-01-01"
end_date = "2021-09-17"
date_col = "感染判明日"
no_col = "No"
age_col = "年齢"
ages = ["10歳未満", "10歳代", "20歳代", "30歳代", "40歳代", "50歳代", "60歳代", "70歳代", "80歳代", "90歳以上"]
ages_label = ["-9", "10-19", "20-29", "30-39", "40-49", "50-59", "60-69", "70-79", "80-89", "90-"]
colors = [
    '#1f77b4',
    '#ff7f0e',
    '#2ca02c',
    '#d62728',
    '#9467bd',
    '#8c564b',
    '#e377c2',
    '#7f7f7f',
    '#bcbd22',
    '#17becf',
]

if __name__ == "__main__":
    df = pandas.read_csv(sys.argv[1], parse_dates=[1])
    raw_data = df[df[date_col] >= pandas.Timestamp(start_date)]
    total = raw_data.filter(items=[no_col, date_col]).groupby(date_col).count()
    groupby_data = raw_data.filter(items=[no_col, date_col, age_col]).groupby([date_col, age_col]).count()[no_col]

    data = {}
    for d in pandas.date_range(start_date, end_date):
        if not d in groupby_data:
            continue
        total = groupby_data[d].sum()
        a = [groupby_data[d].get(a, 0) / total * 100 for a in ages]
        data[d] = a

    plot_data = {}
    for i, a in enumerate(ages):
        plot_data[a] = []
        for d in pandas.date_range(start_date, end_date):
            if not d in data:
                continue
            plot_data[a].append(data[d][i])

    fix, ax = plt.subplots()
    ax.stackplot(list(data.keys()), list(plot_data.values()), labels=ages_label, colors=colors)
    ax.set_title("New cases ratio by age (Akita)")
    ax.set_ylabel("Percentage")
    ax.set_xlabel("Date")
    ax.set_xlim(pandas.Timestamp(start_date), pandas.Timestamp(end_date))
    ax.set_ylim(0, 100)
    ax.legend(loc="upper left")
    plt.show()