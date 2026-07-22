# Rebuilding the paper

```sh
# 1. Regenerate the data-driven figures from benchmarks/ (optional —
#    figures/*.pdf are already committed; only needed if you re-run
#    the benchmarks/demo and want updated numbers/plots)
python3 ../scripts/plot_results.py

# 2. Regenerate the hand-drawn diagrams (optional — diagrams/*.pdf are
#    already committed; only needed if you edit a .tex diagram source)
cd diagrams
for f in architecture sequence address_resolution write_safety pdu_budget; do
  pdflatex -interaction=nonstopmode "$f.tex"
done
cd ..

# 3. Compile the paper (three passes: references, then citations, then
#    final cross-reference resolution)
pdflatex -interaction=nonstopmode otcat.tex
pdflatex -interaction=nonstopmode otcat.tex
pdflatex -interaction=nonstopmode otcat.tex
```

Requires a TeX Live install with `IEEEtran.cls` (Debian/Ubuntu:
`texlive-publishers`), plus `texlive-latex-extra`, `texlive-science`
(algorithmic/algorithm), and `texlive-pictures` (TikZ). All figure and
diagram sources, and the exact commands that produced the data behind
them, are in `../benchmarks/README.md`.

`otcat.tex` targets zero `Overfull \hbox` / `Underfull \vbox` warnings
and zero orphaned section headers (checked by grepping the first/last
lines of every rendered page — see the build log in the project
history for the exact check). If you edit the paper and reintroduce
one, `grep -i overfull otcat.log` after a compile will find it; the fix
is almost always either splitting a long equation line
(`\S\ref{sec:case-study}`'s process-model equations are a worked
example) or switching a narrow table column from a plain `p{}` to the
ragged-right `L{}` column type defined near the top of `otcat.tex` —
justified text in a narrow column is what actually causes most of
these, not the column being nominally "too narrow."
