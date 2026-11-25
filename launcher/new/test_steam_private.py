import sqlite3,tempfile,unittest
from contextlib import closing
from pathlib import Path
from unittest.mock import patch
from client_identity import classify
from capture_import import ensure_uid
from steam_private_runner import validated_config,SUPPORTED_GAME_ASSEMBLY

class SteamRegressionTests(unittest.TestCase):
 def test_steam_wins_over_dormant_channel_dlls(self):
  with tempfile.TemporaryDirectory() as t:
   root=Path(t);p=root/'launcher_Data/Plugins';(p/'x86_64').mkdir(parents=True);(p/'Win/PCSDK_release').mkdir(parents=True)
   for f in ['x86_64/steam_api64.dll','x86_64/PCGameSDK.dll','Win/PCSDK_release/PCSDKUI.exe']:(p/f).write_bytes(b'test')
   self.assertEqual(classify(root),'steam-pc')
 def test_unknown_hash_rejected(self):
  with tempfile.TemporaryDirectory() as t:
   c=Path(t)/'profiles/solo/client';(c/'launcher_Data/Plugins/x86_64').mkdir(parents=True);(c/'launcher_Data/Plugins/x86_64/steam_api64.dll').write_bytes(b'x');(c/'GameAssembly.dll').write_bytes(b'changed')
   with self.assertRaisesRegex(ValueError,'Unverified'):validated_config(c,'127.0.0.1',18080,1)
   with self.assertRaises(ValueError):validated_config(c,'8.8.8.8',18080,1)
 def test_original_client_rejected(self):
  with self.assertRaisesRegex(ValueError,'isolated'):validated_config(Path('D:/SteamLibrary/steamapps/common/OverField'),'127.0.0.1',18080,1)
 def test_nickname_seed_and_preserve(self):
  with tempfile.TemporaryDirectory() as t:
   db=Path(t)/'a.db'
   with closing(sqlite3.connect(db)) as c,c:
    c.execute('CREATE TABLE of_user(user_id INTEGER PRIMARY KEY,sdk_uid INTEGER UNIQUE,ban INTEGER)');c.execute('CREATE TABLE of_game_basic(user_id INTEGER PRIMARY KEY,nick_name TEXT)')
   self.assertEqual(ensure_uid(db,123),123)
   with closing(sqlite3.connect(db)) as c,c:
    self.assertEqual(c.execute('SELECT nick_name FROM of_game_basic').fetchone()[0],'Local123');c.execute("UPDATE of_game_basic SET nick_name='Imported'")
   ensure_uid(db,123)
   with closing(sqlite3.connect(db)) as c,c:self.assertEqual(c.execute('SELECT nick_name FROM of_game_basic').fetchone()[0],'Imported')

if __name__=='__main__':unittest.main()
