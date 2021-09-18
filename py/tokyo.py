import sys
import pandas
import matplotlib.pyplot as plt

start_date = "2021-01-01"
end_date = "2021-09-17"
date_col = "公表_年月日"
no_col = "No"
age_col = "患者_年代"
ages = ["10歳未満", "10代", "20代", "30代", "40代", "50代", "60代", "70代", "80代", "90代", "100歳以上"]
ages_label = ["-9", "10-19", "20-29", "30-39", "40-49", "50-59", "60-69", "70-79", "80-89", "90-99", "100-"]
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
    '#fac205',
]

if __name__ == "__main__":
    df = pandas.read_csv(sys.argv[1], parse_dates=[4])
    raw_data = df[df[date_col] >= pandas.Timestamp(start_date)]
    total = raw_data.filter(items=[no_col, date_col]).groupby(date_col).count()
    groupby_data = raw_data.filter(items=[no_col, date_col, age_col]).groupby([date_col, age_col]).count()[no_col]

    data = {}
    for d in pandas.date_range(start_date, end_date):
        total = groupby_data[d].sum()
        a = [groupby_data[d].get(a, 0) / total * 100 for a in ages]
        data[d] = a

    plot_data = {}
    for i, a in enumerate(ages):
        plot_data[a] = []
        for d in pandas.date_range(start_date, end_date):
            plot_data[a].append(data[d][i])

    fix, ax = plt.subplots()
    ax.stackplot(list(pandas.date_range(start_date, end_date)), list(plot_data.values()), labels=ages_label, colors=colors)
    ax.set_title("Percentage of new cases per day by age")
    ax.set_ylabel("Percentage")
    ax.set_xlabel("Date")
    ax.set_xlim(pandas.Timestamp(start_date), pandas.Timestamp(end_date))
    ax.set_ylim(0, 100)
    ax.legend(loc="upper left")
    plt.show()