"""A live dashboard for one register.

    pip install otcat[dashboard]
    otcat-mockplc --addr 127.0.0.1:15020 &
    streamlit run streamlit_dashboard.py
"""
import time

import pandas as pd
import streamlit as st

from otcat import Client
from otcat.pandas_ext import RollingBuffer

st.set_page_config(page_title="otcat live dashboard", layout="wide")
st.title("otcat live dashboard")

endpoint = st.sidebar.text_input("Modbus endpoint", "127.0.0.1:15020")
spec = st.sidebar.text_input("Address spec", "holding:0")
raw_address = st.sidebar.checkbox("--raw-address", value=True)
window = st.sidebar.slider("Rolling window (samples)", 10, 500, 100)
interval_ms = st.sidebar.slider("Poll interval (ms)", 100, 2000, 500)

if "buffer" not in st.session_state or st.session_state.get("_spec") != spec:
    st.session_state.buffer = RollingBuffer(maxlen=window)
    st.session_state._spec = spec

placeholder = st.empty()
run = st.sidebar.toggle("Running", value=True)

if run:
    client = Client(endpoint, raw_address=raw_address, timeout=2.0)
    try:
        v = client.read(spec)
        st.session_state.buffer.push(v)
    except Exception as e:
        st.sidebar.error(str(e))

    df = st.session_state.buffer.to_dataframe()
    with placeholder.container():
        col1, col2, col3 = st.columns(3)
        if not df.empty:
            col1.metric("latest", df["value"].iloc[-1])
            col2.metric("mean", f"{pd.to_numeric(df['value'], errors='coerce').mean():.2f}")
            col3.metric("quality", df["quality"].iloc[-1])
            st.line_chart(df["value"])
            st.dataframe(df.tail(20), use_container_width=True)
        else:
            st.info("waiting for first reading...")

    time.sleep(interval_ms / 1000)
    st.rerun()
