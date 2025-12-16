Objective: Investigate discrepancies between basis swap spreads calculated by our implementation versus
reference values in the database.

Steps to Reproduce:
1. Execute: cd molib && ./scripts/basis_swap.sh 20251031
2. Compare outputs against database values in pricing.basis_swap

Reference Spreads (Bloomberg SWPM):

BGN EURIBOR3M/EURIBOR6M:
- 10x10: -9.896 bps
- 10x20: -11.573 bps


Task:
1. Locate the calculation logic in airflow/dags/pricing/basis_swap.py and its associated modules
2. Trace the calculation methodology step-by-step
3. Identify where discrepancies occur between our implementation and Bloomberg's results

Database Access:
PGPASSWORD=04201 psql -h 100.127.72.74 -p 1013 -U meenmo -d ficc

Constraints:
- DO NOT modify any files under airflow/ or ficclib/ directories
- Analysis and investigation only

Note: Please ask clarifying questions if any requirements are unclear.