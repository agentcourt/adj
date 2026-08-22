# Iran AIS official NOTAM capture attempt

Fetched from official Iran Aeronautical Information Services public website (`https://ais.airport.ir/85`) during the May 24, 2026 evidence check.

Official page state:

- Page: `https://ais.airport.ir/85`
- Title: `List of valid NOTAM(A and B Series)`
- Page-reported last update: `5/19/26 12:50 PM`
- Documents listed: `SUMMARY_A_19 MAY 2026.pdf`, `SUMMARY_B_19 MAY 2026.pdf`

Captured official PDFs:

- `SUMMARY_A_19_MAY_2026.official.pdf`
  - Direct URL: `https://ais.airport.ir/documents/452631/25995275/SUMMARY_A_19+MAY+2026.pdf/53ed5a73-c62c-456c-bfc4-ddf36a649f65?version=1.0`
  - SHA-256: `79d4d56209ea154d8f6c958f651314fad59340a0bb53b631d3dbbdb002429097`
  - Extracted text: `SUMMARY_A_19_MAY_2026.official.txt`
- `SUMMARY_B_19_MAY_2026.official.pdf`
  - Direct URL: `https://ais.airport.ir/documents/452631/25995275/SUMMARY_B_19+MAY+2026.pdf/9bf83540-3efe-4e30-b2d9-f571f0a132f0?version=1.0`
  - SHA-256: `d6c6ecc68df67138face3233dd534f158e1a5d256bfb5b8fb961881c9db1f25a`
  - Extracted text: `SUMMARY_B_19_MAY_2026.official.txt`

Relevant official Series A excerpts found in `SUMMARY_A_19_MAY_2026.official.txt`:

```text
A0796 260421
B) 2604210949            C) 2605250830EST
E) TEHRAN FIR RESUMED NORMAL OPERATION, WHILE WEST PART REMAINS
CLOSED FOR OVERFLIGHTS.
DIVIDING LINE GOES THROUGH IVIVA-PURBO-ANK-OBRIX THEN CLOCKWISE
ALONG TEHRAN TMA TO NAGMO AND LALDA.
...
A0799 260421
B) 2604211010            C) 2605250830EST
E) REF NOTAM  A0796/26, ALL AIRPORTS IN WEST PART OF TEHRAN FIR ARE CLSD
EXCEPT OITR, OIKK, OIAA, OISS, OIYY, OICC AND OIGG THAT ARE
OPERATIONAL FM SUNRISE TO SUNSET (HJ).
ALL PREVIOUS PERMISSIONS ARE SUSPENDED FOR ALL OPERATORS,
NEW PERMISSION SHALL BE REQUESTED FM CAA FOR OPRATION OF
CIVIL IFR PASSENGER FLIGHTS ON THESE AIRPORTS.
...
A0946 260511
B) 2605111720            C) 2605250830EST
E) REF NOTAM A0796/26, RAGET AND PAXAT ARE AVAILABLE AS TRANSFER OF CONTROL
POINTS BTN TEHRAN FIR AND BAGHDAD FIR FM 0230-1430 UTC,
ONLY FOR FLIGHTS ARRIVING TO OR DEPARTING FM AIRPORTS IN TEHRAN FIR
```

Result:

- The official public Iran AIS list corroborates A0796/26, A0799/26, and A0946/26 as of the 19 May 2026 list.
- It does not contain A1010/26 because the public page was still showing the 19 May list at the time checked.
- A1010/26 is a later `NOTAMR A0799/26` from 22 May according to the mirrored NOTAM evidence. The official public Iran AIS site did not expose a later `SUMMARY_A` PDF through the visible document library during this check.
