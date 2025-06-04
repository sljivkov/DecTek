import pytest
import requests
from pydantic import BaseModel, Field
from decimal import Decimal

BASE_URL = "http://localhost:8080"


class PriceEntry(BaseModel):
    symbol: str = Field(alias="Symbol")
    amount: Decimal = Field(alias="Amount", gt=0)
    type: str = Field(alias="Type")


def test_get_prices():
    """Test /prices endpoint returns valid data."""
    response = requests.get(f"{BASE_URL}/prices")
    assert response.status_code == 200
    
    # Validate response structure with Pydantic
    entries = [PriceEntry(**entry) for entry in response.json()]
    
    # Response can be empty if price feed hasn't started yet
    if entries:
        # Check we have bitcoin and ethereum
        symbols = {e.symbol.lower() for e in entries}
        assert {"bitcoin", "ethereum"}.issubset(symbols)
        
        # Check we have USD and EUR
        currencies = {e.type for e in entries}
        assert {"USD", "EUR"}.issubset(currencies)


def test_set_price_errors():
    """Test validation errors."""
    bad_data = [
        {"symbol": "bitcoin", "type": "USD"},  # no amount
        {"symbol": "bitcoin", "amount": -100, "type": "USD"},  # negative
        {"symbol": "", "amount": 100, "type": "USD"},  # empty symbol
    ]
    
    for payload in bad_data:
        response = requests.post(f"{BASE_URL}/set-price", json=payload)
        assert response.status_code == 400


@pytest.mark.parametrize("payload, expected_status_code", [
    ({"symbol": "", "amount": 100, "type": "USD"}, 400),  # empty symbol
    ({"symbol": "bitcoin", "amount": -100, "type": "USD"}, 400),  # negative amount
    ({"symbol": "bitcoin", "amount": "abc", "type": "USD"}, 400),  # non-numeric amount
    ({"symbol": "unknown", "amount": 100, "type": "USD"}, 404),  # unknown symbol
    ({"symbol": "bitcoin", "amount": 100}, 400),  # missing type
    ({"symbol": "bitcoin", "amount": 100, "type": "ABC"}, 400),  # invalid type
])
def test_set_price(payload, expected_status_code):
    response = requests.post(f"{BASE_URL}/set-price", json=payload)
    assert response.status_code == expected_status_code