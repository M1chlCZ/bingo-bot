import os, math, datetime
import sqlite3
import numpy as np
import pandas as pd

import tensorflow as tf
from tensorflow import keras
from tensorflow.keras import layers, regularizers
from tensorflow.keras.callbacks import EarlyStopping

# For hyperparameter tuning
import keras_tuner as kt

from sklearn.model_selection import KFold, train_test_split
from sklearn.preprocessing import StandardScaler
from sklearn.metrics import accuracy_score, classification_report
from sklearn.utils import shuffle

from joblib import dump

#####################
#   CONFIG
#####################
# Paths to your DBs
DB_PATHS = [
    '/home/konomut/.trading/trades.db',
]

#####################
#  1) LOAD & MERGE
#####################

def load_merged_data():
    """
    Loads data from multiple SQLite databases, returning a single DataFrame.

    Adjust the SELECT query to match your table schema.
    For instance, if you have columns:
      - buy_price, sell_price
      - rsi, macd, stochastic
      - cci, mfi, upperBound, middleBound, lowerBound, ichimoku_tenkan, ichimoku_kijun
      - profit_loss
    ... you can add them to the SELECT below.

    We print debug logs about how many rows are loaded from each DB.
    """
    all_data = []
    for db_path in DB_PATHS:
        if not os.path.exists(db_path):
            print(f"[DEBUG] DB path does not exist: {db_path}")
            continue

        try:
            conn = sqlite3.connect(db_path)
            query = """
            SELECT
        a.order_id,
        a.symbol,
        a.timestamp as buy_timestamp,
        a.rsi as rsi_buy,
        a.macd as macd_buy,
        a.cci as cci_buy,
        a.mfi as mfi_buy,
        a.quantity as volume_buy,
        a.middleBound as middleBand_buy,
        a.upperBound as upperBand_buy,
        a.lowerBound as lowerBand_buy,
        c.profit_loss,
        c.timestamp as sell_timestamp
    FROM active_trades a
    JOIN completed_trades c
       ON a.order_id = c.order_id
    WHERE rsi_buy != 0
      AND upperBand_buy != 0
      AND profit_loss != 0
            """
            df = pd.read_sql_query(query, conn)
            conn.close()

            print(f"[DEBUG] Loaded {len(df)} rows from {db_path}")
            if not df.empty:
                all_data.append(df)
        except Exception as e:
            print(f"[ERROR] loading data from {db_path}: {e}")

    if len(all_data) == 0:
        print("[DEBUG] No data loaded from any DBs.")
        return pd.DataFrame()

    merged = pd.concat(all_data, ignore_index=True)
    print(f"[DEBUG] Merged data shape after loading: {merged.shape}")
    return merged

#####################
#  2) FEATURE ENGINEERING
#####################

def feature_engineering(df):
    """
    We add some 'wild' example features:
    1) Bollinger distance ratio: (middleBand - lowerBand)/(upperBand - lowerBand)
    2) Volume ratio: volume_buy / ?

    We also parse time-of-day from buy_timestamp for cyclical encoding.
    (If your timestamp is a string, parse to datetime, etc.)
    Then we create a final label => profit_label=1 if profit_loss>0 else 0
    """

    df = df.dropna().copy()

    # 1) Bollinger distance ratio
    df['bb_ratio'] = (df['middleBand_buy'] - df['lowerBand_buy']) / (
        (df['upperBand_buy'] - df['lowerBand_buy']) + 1e-9
    )

    # 2) Volume ratio - this is arbitrary if we had some average volume
    # We'll do a small hack: ratio vs. standard threshold, or just keep volume as is
    df['volume_log'] = np.log(df['volume_buy'] + 1e-9)

    # 3) Time-of-day from buy_timestamp
    # If buy_timestamp is a string "2024-12-10 02:09:17", we parse it:
    def get_hour_of_day(ts):
        # e.g. "2024-12-10 02:09:17"
        dt = datetime.datetime.strptime(ts, "%Y-%m-%d %H:%M:%S")
        return dt.hour

    df['buy_hour'] = df['buy_timestamp'].apply(get_hour_of_day)
    # We can do cyclical encoding => sin/cos
    df['buy_hour_sin'] = np.sin(2 * math.pi * df['buy_hour'] / 24.0)
    df['buy_hour_cos'] = np.cos(2 * math.pi * df['buy_hour'] / 24.0)

    # 4) Final label => profit_label
    df['profit_label'] = (df['profit_loss'] > 0).astype(int)

    return df

#####################
#  3) BUILD & TUNE KERAS MODEL
#####################

def build_model(hp):
    """
    This function builds a Keras model w/ hyperparameters from keras-tuner.
    We'll do a small search for:
      - number of units in layer 1 (32-128)
      - number of units in layer 2 (16-64)
      - dropout (0.2 - 0.5)
      - L2 regularization factor
      - learning rate
    """
    model = keras.Sequential()

    # Input dimension is unspecified here; we set it after we create the tuner.

    # First layer
    units_1 = hp.Int('units_1', min_value=32, max_value=128, step=32)
    model.add(layers.Dense(
        units_1, 
        activation='relu', 
        kernel_regularizer=regularizers.l2(hp.Float('l2_1', 1e-6, 1e-3, sampling='log'))
    ))
    model.add(layers.Dropout(hp.Float('dropout_1', 0.2, 0.5, step=0.1)))

    # Second layer
    units_2 = hp.Int('units_2', min_value=16, max_value=64, step=16)
    model.add(layers.Dense(
        units_2, 
        activation='relu', 
        kernel_regularizer=regularizers.l2(hp.Float('l2_2', 1e-6, 1e-3, sampling='log'))
    ))
    model.add(layers.Dropout(hp.Float('dropout_2', 0.2, 0.5, step=0.1)))

    # Output layer => 1 (sigmoid)
    model.add(layers.Dense(1, activation='sigmoid'))

    # Learning rate
    lr = hp.Float('lr', 1e-4, 1e-2, sampling='log')
    optimizer = keras.optimizers.Adam(learning_rate=lr)
    model.compile(
        optimizer=optimizer, 
        loss='binary_crossentropy',
        metrics=['accuracy']
    )
    return model

def run_kfold_cv(X, y, n_splits=5):
    """
    We'll do a KFold CV to get a sense of performance. 
    For each split, we do a brand-new model from the best hyperparams found. 
    In practice, you might want to do a single hyperparam search outside, 
    then fix them and do KFold. 
    But let's do a simplified approach for demonstration.
    """
    kf = KFold(n_splits=n_splits, shuffle=True, random_state=42)
    accuracies = []

    for fold, (train_idx, test_idx) in enumerate(kf.split(X, y)):
        print(f"\n[INFO] KFold fold={fold+1}/{n_splits}")
        X_train, X_test = X.iloc[train_idx], X.iloc[test_idx]
        y_train, y_test = y.iloc[train_idx], y.iloc[test_idx]

        # Scale
        scaler = StandardScaler()
        X_train_s = scaler.fit_transform(X_train)
        X_test_s  = scaler.transform(X_test)

        # --- TUNE HYPERPARAMS ---
        tuner = kt.RandomSearch(
            build_model,
            objective='val_accuracy',
            max_trials=5,  # you can increase for more thorough search
            executions_per_trial=1,
            overwrite=True,
            directory='ktuner_dir',
            project_name=f'fold_{fold}'
        )

        stop_early = keras.callbacks.EarlyStopping(monitor='val_loss', patience=3, restore_best_weights=True)

        tuner.search(
            X_train_s, y_train,
            epochs=20,
            validation_split=0.2,
            callbacks=[stop_early],
            verbose=0
        )

        best_hp = tuner.get_best_hyperparameters(num_trials=1)[0]
        print("[DEBUG] Best HP for this fold:", best_hp.values)

        # Build the best model and train
        model = tuner.hypermodel.build(best_hp)
        history = model.fit(
            X_train_s, y_train,
            epochs=20,
            validation_split=0.2,
            callbacks=[stop_early],
            verbose=0
        )

        # Evaluate
        loss, acc = model.evaluate(X_test_s, y_test, verbose=0)
        print(f"[INFO] Fold={fold+1}, test accuracy={acc*100:.2f}%")
        accuracies.append(acc)
    
    print("\n=== CROSS-VAL RESULTS ===")
    print("Accuracies:", [f"{a*100:.2f}%" for a in accuracies])
    print(f"Mean acc={np.mean(accuracies)*100:.2f}%, std={np.std(accuracies)*100:.2f}%")

#####################
# FINAL MODEL TRAINING & SAVE
#####################

def train_final_model(X, y):
    """
    This function trains on the ENTIRE dataset with best hyperparams found by a separate tuner search
    and then saves the final model + scaler.
    For demonstration, we do a new tuner search on all data or pick some fold's best HP.
    """
    # Shuffle data
    X, y = shuffle(X, y, random_state=42)

    # We can do a new tuner search on ALL data with a train/val split
    # or we can skip to use a known best HP from K-fold. 
    # For demonstration, let's do a new tuner search on entire data.

    # Train/val split
    X_train, X_val, y_train, y_val = train_test_split(
        X, y, test_size=0.2, random_state=42, stratify=y
    )

    # Scale
    scaler = StandardScaler()
    X_train_s = scaler.fit_transform(X_train)
    X_val_s   = scaler.transform(X_val)

    best_hp = do_hyperparam_search(X_train_s, y_train, fold_name="final_all_data")
    print("[DEBUG] best_hp final training =>", best_hp.values)

    # Build final model
    model = build_model(best_hp)
    stop_early = EarlyStopping(monitor='val_loss', patience=3, restore_best_weights=True)

    # Fit
    model.fit(
        X_train_s, y_train,
        validation_data=(X_val_s, y_val),
        epochs=20,
        callbacks=[stop_early],
        verbose=1
    )

    # Evaluate on val set
    val_loss, val_acc = model.evaluate(X_val_s, y_val, verbose=0)
    print(f"[INFO] Final Model Val Accuracy => {val_acc*100:.2f}%")

    # Save model
    model.save("buy_outcome_keras_model")
    print("[INFO] Keras model saved to buy_outcome_keras_model/")
    dump(scaler, "buy_outcome_keras_model_scaler.joblib")
    print("[INFO] Saved scaler to buy_outcome_keras_model_scaler.joblib")

def main():
    df = load_merged_data()
    if df.empty:
        print("[ERROR] No data loaded => exit.")
        return

    df = feature_engineering(df)
    if df.empty:
        print("[ERROR] After feature eng => no data => exit.")
        return

    # We pick some columns for final training
    feature_cols = [
        'rsi_buy',
        'macd_buy',
        'cci_buy',
        'mfi_buy',
        'bb_ratio',
        'volume_log',
        'buy_hour_sin',
        'buy_hour_cos',
    ]

    # We do a binary label => profit_label
    X = df[feature_cols].copy()
    y = df['profit_label'].copy()


    # K-Fold + Keras Tuner
    run_kfold_cv(X, y, n_splits=3)

    # Final training on the entire dataset + save
    train_final_model(X, y)

if __name__ == "__main__":
    main()
