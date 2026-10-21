import asyncio
import pytest
from voice.floor import Availability, PartnerAvailability
from test_voice_device_floor_integration import setup, path, payload, finish, play_all

async def test_rejected_decision_has_no_delayed_bridge_input(tmp_path):
    app,a,b,bridge,_=setup(tmp_path)
    async with a,b:
        room=app.state.room
        room.registry.availability=lambda busy=(): [PartnerAvailability('gge',Availability.BUSY),PartnerAvailability('cursor',Availability.AVAILABLE)]
        response=await a.post(path('gge')+'/turn',json=payload())
        await finish(app)
        await asyncio.sleep(.05)
        assert response.status_code==409
        assert not [call for call in bridge.calls if call[0]=='/input']
        assert not bridge.requests and not app.state.service.turns and not room.slots

async def test_cross_device_answers_play_in_admission_order(tmp_path):
    app,a,b,bridge,_=setup(tmp_path)
    async with a,b:
        first,second=payload(),payload('cursor')
        assert (await a.post(path('gge')+'/turn',json=first)).status_code==202
        await finish(app)
        assert (await b.post(path('cursor')+'/turn',json=second)).status_code==202
        await finish(app)
        assert (await a.get(path('gge',first['id']))).json()['playback_allowed']
        assert not (await b.get(path('cursor',second['id']))).json()['playback_allowed']
        # No bypass using a guessed chunk URL while waiting.
        assert (await b.get(path('cursor',second['id'])+'/audio/chunks/0')).status_code==409
        await play_all(a,'gge',first)
        assert (await b.get(path('cursor',second['id']))).json()['playback_allowed']
        await play_all(b,'cursor',second)
        assert [body['project'] for route,body in bridge.calls if route=='/input']==['gge','cursor']

